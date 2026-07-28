package solana

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
				"The Solana test network requires the transmitter account (derived from your private key) to exist and be funded on-chain.\n\n" +
				"If you already have a Solana CLI keypair, point the variable at the file:\n\n" +
				"  CRE_SOLANA_PRIVATE_KEY=~/.config/solana/id.json\n\n" +
				"To create one:\n\n" +
				"  solana-keygen new\n" +
				"  solana airdrop 2 --url devnet\n\n" +
				"Fund the account on the same cluster your RPC points at; an account funded on\n" +
				"mainnet is invisible to a devnet simulation (it fails with AccountNotFound).\n" +
				"Check with: solana balance --url devnet\n\n" +
				"and then point the variable at the file:\n\n" +
				"  CRE_SOLANA_PRIVATE_KEY=~/.config/solana/id.json\n\n" +
				"A base58-encoded 64-byte keypair is also accepted:\n\n" +
				"  CRE_SOLANA_PRIVATE_KEY=4wBqpZM9xaSheZzJSMawUHDgZ7miWfSsxmeRUJ1s...",
		)
	}

	key, err := parseSolanaKey(raw)
	if err != nil {
		return nil, err
	}
	if broadcast && key.PublicKey().IsZero() {
		return nil, fmt.Errorf("CRE_SOLANA_PRIVATE_KEY decodes to a zero key; refusing to broadcast")
	}
	return key, nil
}

// parseSolanaKey accepts the three shapes a user is likely to have on hand:
// the contents of a `solana-keygen` keyfile pasted inline, a base58-encoded
// 64-byte keypair, or a path to a keyfile. `solana-keygen new` writes the JSON
// byte-array form and has no flag to print the base58 secret, so requiring
// base58 alone leaves users with no way to use a freshly generated key.
// Base58 is tried before the path interpretation so existing configs resolve
// exactly as they did before, and no path-shape guessing is needed.
func parseSolanaKey(raw string) (solana.PrivateKey, error) {
	if strings.HasPrefix(raw, "[") {
		key, err := solana.PrivateKeyFromSolanaKeygenFileBytes([]byte(raw))
		if err != nil {
			return nil, keyFormatError(err)
		}
		return key, nil
	}

	if key, err := solana.PrivateKeyFromBase58(raw); err == nil {
		return key, nil
	}

	path, err := expandHome(raw)
	if err != nil {
		return nil, keyFormatError(err)
	}
	key, err := solana.PrivateKeyFromSolanaKeygenFile(path)
	if err != nil {
		return nil, keyFormatError(err)
	}
	return key, nil
}

func keyFormatError(err error) error {
	return fmt.Errorf(
		"CRE_SOLANA_PRIVATE_KEY must be a base58-encoded 64-byte keypair, a path to a "+
			"solana-keygen keyfile (e.g. ~/.config/solana/id.json), or that file's JSON contents: %w", err)
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}

// ResolveTriggerData fetches the Solana log payload for the given selector by
// replaying a known transaction.
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
	// Unlike EVM, Solana has no live-listen fallback: --solana-tx-sig and
	// --solana-event-index are always required for Solana log triggers.
	if sig == "" || eventIndexStr == "" {
		return nil, fmt.Errorf("--%s and --%s are required for Solana log triggers", TriggerInputTxSig, TriggerInputEventIndex)
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
	return GetSolanaTriggerLogWithFilter(ctx, client, sig, eventIndex, filter)
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
