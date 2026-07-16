package solana

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	crpc "github.com/smartcontractkit/cre-cli/internal/rpc"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// CLI input keys consumed from chain.TriggerParams.ChainTypeInputs.
const (
	TriggerInputTxSig      = "solana-tx-sig"
	TriggerInputEventIndex = "solana-event-index"
)

func init() {
	chain.Register(string(corekeys.Solana), func(lggr *zerolog.Logger) chain.ChainType {
		return &SolanaChainType{log: lggr}
	}, []chain.CLIFlagDef{
		{Name: TriggerInputTxSig, Description: "Solana trigger transaction signature (base58)", FlagType: chain.CLIFlagString},
		{Name: TriggerInputEventIndex, Description: "Solana trigger event index (0-based, among 'Program data:' events in the tx)", DefaultValue: "-1", FlagType: chain.CLIFlagInt},
	})
}

// SolanaChainType implements chain.ChainType for Solana.
type SolanaChainType struct {
	log           *zerolog.Logger
	solanaChains  *SolanaChainCapabilities
	programIDs    map[uint64]solana.PublicKey
	stateAccounts map[uint64]solana.PublicKey
}

var _ chain.ChainType = (*SolanaChainType)(nil)

func (ct *SolanaChainType) Name() string                         { return string(corekeys.Solana) }
func (ct *SolanaChainType) SupportedChains() []chain.ChainConfig { return SupportedChains }

func (ct *SolanaChainType) ResolveClients(v *viper.Viper) (chain.ResolvedChains, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)
	ct.programIDs = make(map[uint64]solana.PublicKey)
	ct.stateAccounts = make(map[uint64]solana.PublicKey)

	for _, c := range SupportedChains {
		name, err := settings.GetChainNameByChainSelector(c.Selector)
		if err != nil {
			ct.log.Error().Msgf("Invalid Solana chain selector %d; skipping", c.Selector)
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, name)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			ct.log.Debug().Msgf("RPC not provided for %s; skipping", name)
			continue
		}
		ct.log.Debug().Msgf("Using RPC for %s: %s", name, crpc.RedactURL(rpcURL))

		programID, err := solana.PublicKeyFromBase58(c.Forwarder)
		if err != nil {
			return chain.ResolvedChains{}, fmt.Errorf("invalid forwarder program ID for %s: %w", name, err)
		}
		stateB58, ok := forwarderStateAccounts[c.Selector]
		if !ok || strings.TrimSpace(stateB58) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("no forwarder state account configured for %s", name)
		}
		state, err := solana.PublicKeyFromBase58(stateB58)
		if err != nil {
			return chain.ResolvedChains{}, fmt.Errorf("invalid forwarder state account for %s: %w", name, err)
		}

		clients[c.Selector] = solanarpc.New(rpcURL)
		forwarders[c.Selector] = c.Forwarder
		ct.programIDs[c.Selector] = programID
		ct.stateAccounts[c.Selector] = state
	}

	return chain.ResolvedChains{Clients: clients, Forwarders: forwarders}, nil
}

func (ct *SolanaChainType) ResolveKey(s *settings.Settings, broadcast bool) (interface{}, error) {
	raw := strings.TrimSpace(s.User.PrivateKey(settings.Solana))

	// Solana simulation requires a valid private key in all cases (both broadcast and non-broadcast).
	// Unlike EVM (which uses Anvil with pre-funded deterministic accounts), Solana's test network
	// requires the transmitter account to exist and be funded on-chain. Using a random or sentinel key
	// will fail when the RPC tries to access a non-existent signer account.
	// Solution: Mandate CRE_SOLANA_PRIVATE_KEY for all Solana workflow simulations.
	if raw == "" {
		return nil, fmt.Errorf(
			"CRE_SOLANA_PRIVATE_KEY is required for Solana workflow simulation.\n\n" +
				"The Solana test network requires the transmitter account (derived from your private key) to exist and be funded on-chain.\n" +
				"Please set your private key in your .env file or system environment:\n\n" +
				"  CRE_SOLANA_PRIVATE_KEY=<your-64-byte-base58-keypair>\n\n" +
				"You can generate a test key using: solana-keygen new\n" +
				"Then fund it on devnet: solana airdrop 10 <your-address> --url devnet",
		)
	}

	// Try base58 (64-byte solana keypair, standard Solana CLI / wallet format).
	if key, err := solana.PrivateKeyFromBase58(raw); err == nil && len(key) == 64 {
		if broadcast && key.PublicKey().IsZero() {
			return nil, fmt.Errorf("CRE_SOLANA_PRIVATE_KEY decodes to a zero key; refusing to broadcast")
		}
		return key, nil
	}

	return nil, fmt.Errorf("CRE_SOLANA_PRIVATE_KEY must be a 64-byte base58 keypair")
}

// ResolveTriggerData fetches the Solana log payload for the given selector by
// replaying a known transaction (--solana-tx-sig + --solana-event-index).
// Replay works in both interactive and non-interactive modes for deterministic
// testing / CI. There is no live-listen path yet.
func (ct *SolanaChainType) ResolveTriggerData(ctx context.Context, selector uint64, params chain.TriggerParams) (interface{}, error) {
	clientIface, ok := params.Clients[selector]
	if !ok {
		return nil, fmt.Errorf("no RPC configured for chain selector %d", selector)
	}
	client, ok := clientIface.(*solanarpc.Client)
	if !ok {
		return nil, fmt.Errorf("invalid client type for Solana chain selector %d", selector)
	}

	sig := strings.TrimSpace(params.ChainTypeInputs[TriggerInputTxSig])
	eventIndexStr := strings.TrimSpace(params.ChainTypeInputs[TriggerInputEventIndex])
	if sig == "" {
		return nil, fmt.Errorf("--%s and --%s are required for Solana log triggers", TriggerInputTxSig, TriggerInputEventIndex)
	}
	if eventIndexStr == "" {
		return nil, fmt.Errorf("--%s is required when --%s is provided", TriggerInputEventIndex, TriggerInputTxSig)
	}
	eventIndex, err := strconv.ParseUint(eventIndexStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid --%s %q: %w", TriggerInputEventIndex, eventIndexStr, err)
	}

	if params.Interactive {
		printSolanaTriggerReplayHeader(selector, sig, eventIndex)
	}

	var filter *solcap.FilterLogTriggerRequest
	if params.TriggerPayload != nil {
		filter, err = decodeLogTriggerConfig(params.TriggerPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode Solana log trigger config: %w", err)
		}
	}
	return GetSolanaTriggerLogFromValuesWithFilter(ctx, client, sig, eventIndex, filter)
}

func (ct *SolanaChainType) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) ([]services.Service, error) {
	typedClients := make(map[uint64]*solanarpc.Client, len(cfg.Clients))
	for sel, c := range cfg.Clients {
		sc, ok := c.(*solanarpc.Client)
		if !ok {
			return nil, fmt.Errorf("solana: client for selector %d is not *rpc.Client", sel)
		}
		typedClients[sel] = sc
	}
	var key solana.PrivateKey
	if cfg.PrivateKey != nil {
		var ok bool
		key, ok = cfg.PrivateKey.(solana.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("solana: private key is not solana.PrivateKey")
		}
	}
	var lim chain.Limits
	if cfg.Limits != nil {
		lim = ExtractLimits(cfg.Limits)
	}
	caps, err := NewSolanaChainCapabilities(
		ctx, cfg.Logger, cfg.Registry,
		typedClients,
		ct.programIDs,
		ct.stateAccounts,
		key,
		!cfg.Broadcast,
		lim,
	)
	if err != nil {
		return nil, err
	}
	if err := caps.Start(ctx); err != nil {
		return nil, fmt.Errorf("solana: failed to start: %w", err)
	}
	ct.solanaChains = caps
	out := make([]services.Service, 0, len(caps.SolanaChains))
	for _, fc := range caps.SolanaChains {
		out = append(out, fc)
	}
	return out, nil
}

func (ct *SolanaChainType) ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error {
	if ct.solanaChains == nil {
		return fmt.Errorf("solana: capabilities not registered")
	}
	solanaChain := ct.solanaChains.SolanaChains[selector]
	if solanaChain == nil {
		return fmt.Errorf("no Solana chain initialized for selector %d", selector)
	}
	log, ok := triggerData.(*solcap.Log)
	if !ok {
		return fmt.Errorf("solana: trigger data is not *solana.Log")
	}
	return solanaChain.ManualTrigger(ctx, registrationID, log)
}

func (ct *SolanaChainType) Supports(selector uint64) bool {
	if ct.solanaChains == nil {
		return false
	}
	return ct.solanaChains.SolanaChains[selector] != nil
}

func (ct *SolanaChainType) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	return chain.ParseTriggerChainSelector(ct.Name(), triggerID)
}

func (ct *SolanaChainType) RunHealthCheck(resolved chain.ResolvedChains) error {
	return RunRPCHealthCheck(resolved.Clients, resolved.ExperimentalSelectors)
}

func (ct *SolanaChainType) CollectCLIInputs(v *viper.Viper) map[string]string {
	inputs := map[string]string{}
	if sig := strings.TrimSpace(v.GetString(TriggerInputTxSig)); sig != "" {
		inputs[TriggerInputTxSig] = sig
	}
	if idx := v.GetInt(TriggerInputEventIndex); idx >= 0 {
		inputs[TriggerInputEventIndex] = strconv.Itoa(idx)
	}
	return inputs
}

func ExtractLimits(w *cresettings.Workflows) chain.Limits {
	return chain.Limits{
		ReportSize: int(w.ChainWrite.Solana.ReportSizeLimit.DefaultValue),
		// Solana compute-unit limit is Setting[uint32]; widen to chain.Limits.GasLimit (uint64).
		GasLimit: uint64(w.ChainWrite.Solana.GasLimit.Default.DefaultValue),
	}
}
