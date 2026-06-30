package solana

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// 32 zero bytes → derives ed25519 sentinel keypair. Used when the user hasn't
// configured CRE_SOLANA_PRIVATE_KEY and isn't broadcasting.
const defaultSentinelSolanaSeed = "0000000000000000000000000000000000000000000000000000000000000000"

func init() {
	chain.Register(string(corekeys.Solana), func(lggr *zerolog.Logger) chain.ChainType {
		return &SolanaChainType{log: lggr}
	}, nil)
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
	experimental := make(map[uint64]bool)
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
		ct.log.Debug().Msgf("Using RPC for %s: %s", name, chain.RedactURL(rpcURL))

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

		clients[c.Selector] = rpc.New(rpcURL)
		forwarders[c.Selector] = c.Forwarder
		ct.programIDs[c.Selector] = programID
		ct.stateAccounts[c.Selector] = state
	}

	expChains, err := settings.GetExperimentalChains(v)
	if err != nil {
		return chain.ResolvedChains{}, fmt.Errorf("failed to load experimental chains config: %w", err)
	}
	for _, ec := range expChains {
		if !strings.EqualFold(ec.ChainType, ct.Name()) {
			continue
		}
		if ec.ChainSelector == 0 {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain missing chain-selector")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental solana chain %d missing rpc-url", ec.ChainSelector)
		}
		forwarder := strings.TrimSpace(ec.Forwarder)
		if forwarder == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental solana chain %d missing forwarder", ec.ChainSelector)
		}
		// Experimental Solana entries don't carry a state-account slot; for now we
		// only support experimental chains whose program ID and state are already
		// in the built-in maps. (Once the experimental-chains config grows a
		// chain-specific extra blob, plumb state-account through here.)
		programID, perr := solana.PublicKeyFromBase58(forwarder)
		if perr != nil {
			return chain.ResolvedChains{}, fmt.Errorf("invalid forwarder for experimental solana chain %d: %w", ec.ChainSelector, perr)
		}
		stateB58, hasState := forwarderStateAccounts[ec.ChainSelector]
		if !hasState {
			return chain.ResolvedChains{}, fmt.Errorf("experimental solana chain %d has no built-in state account; not yet supported", ec.ChainSelector)
		}
		state, sErr := solana.PublicKeyFromBase58(stateB58)
		if sErr != nil {
			return chain.ResolvedChains{}, fmt.Errorf("invalid state account for experimental solana chain %d: %w", ec.ChainSelector, sErr)
		}

		if _, exists := clients[ec.ChainSelector]; exists {
			if forwarders[ec.ChainSelector] != forwarder {
				ui.Warning(fmt.Sprintf("Warning: experimental solana chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n",
					ec.ChainSelector, forwarders[ec.ChainSelector], forwarder))
				forwarders[ec.ChainSelector] = forwarder
				ct.programIDs[ec.ChainSelector] = programID
			} else {
				ct.log.Debug().Uint64("chain-selector", ec.ChainSelector).Msg("Experimental chain matches supported chain config")
			}
			continue
		}
		ct.log.Debug().Msgf("Using RPC for experimental solana chain %d: %s", ec.ChainSelector, chain.RedactURL(ec.RPCURL))
		clients[ec.ChainSelector] = rpc.New(ec.RPCURL)
		forwarders[ec.ChainSelector] = forwarder
		ct.programIDs[ec.ChainSelector] = programID
		ct.stateAccounts[ec.ChainSelector] = state
		experimental[ec.ChainSelector] = true
		ui.Dim(fmt.Sprintf("Added experimental solana chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	return chain.ResolvedChains{Clients: clients, Forwarders: forwarders, ExperimentalSelectors: experimental}, nil
}

func (ct *SolanaChainType) ResolveKey(s *settings.Settings, broadcast bool) (interface{}, error) {
	raw := strings.TrimSpace(s.User.PrivateKey(settings.Solana))

	// Empty → sentinel (unless broadcasting).
	if raw == "" {
		if broadcast {
			return nil, fmt.Errorf("CRE_SOLANA_PRIVATE_KEY is required to broadcast. Please set it in your .env file or system environment")
		}
		ui.Warning("Using default Solana private key for chain write simulation. To use your own key, set CRE_SOLANA_PRIVATE_KEY in your .env file or system environment.")
		seed, _ := hex.DecodeString(defaultSentinelSolanaSeed)
		return solana.PrivateKey(ed25519.NewKeyFromSeed(seed)), nil
	}

	// Try base58 (64-byte solana keypair, standard Solana CLI / wallet format) first.
	if key, err := solana.PrivateKeyFromBase58(raw); err == nil && len(key) == 64 {
		if broadcast && key.PublicKey().IsZero() {
			return nil, fmt.Errorf("CRE_SOLANA_PRIVATE_KEY decodes to a zero key; refusing to broadcast")
		}
		return key, nil
	}

	// Fall back to 32-byte hex seed (parity with Aptos input convention).
	hexSeed := strings.TrimPrefix(strings.ToLower(raw), "0x")
	seed, err := hex.DecodeString(hexSeed)
	if err != nil || len(seed) != 32 {
		return nil, fmt.Errorf(
			"CRE_SOLANA_PRIVATE_KEY must be a 64-byte base58 keypair or a 32-byte hex seed; got %d bytes via base58 / %d bytes via hex",
			0, len(seed),
		)
	}
	return solana.PrivateKey(ed25519.NewKeyFromSeed(seed)), nil
}

func (ct *SolanaChainType) ResolveTriggerData(_ context.Context, _ uint64, _ chain.TriggerParams) (interface{}, error) {
	return nil, fmt.Errorf("solana: no trigger surface")
}

func (ct *SolanaChainType) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) ([]services.Service, error) {
	typedClients := make(map[uint64]*rpc.Client, len(cfg.Clients))
	for sel, c := range cfg.Clients {
		sc, ok := c.(*rpc.Client)
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

func (ct *SolanaChainType) ExecuteTrigger(_ context.Context, _ uint64, _ string, _ interface{}) error {
	return fmt.Errorf("solana: no trigger surface")
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

func (ct *SolanaChainType) CollectCLIInputs(_ *viper.Viper) map[string]string {
	return map[string]string{}
}
