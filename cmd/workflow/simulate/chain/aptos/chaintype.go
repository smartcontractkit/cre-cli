package aptos

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/aptos-labs/aptos-go-sdk/crypto"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	"github.com/smartcontractkit/chainlink-common/pkg/services"

	aptosfakes "github.com/smartcontractkit/chainlink-aptos/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const defaultSentinelAptosSeed = "0000000000000000000000000000000000000000000000000000000000000001"

func init() {
	chain.Register(string(corekeys.Aptos), func(lggr *zerolog.Logger) chain.ChainType {
		return &AptosChainType{log: lggr}
	}, nil)
}

// AptosChainType implements chain.ChainType for Aptos.
type AptosChainType struct {
	log         *zerolog.Logger
	aptosChains *AptosChainCapabilities
}

var _ chain.ChainType = (*AptosChainType)(nil)

func (ct *AptosChainType) Name() string                         { return string(corekeys.Aptos) }
func (ct *AptosChainType) SupportedChains() []chain.ChainConfig { return SupportedChains }

func (ct *AptosChainType) ResolveClients(v *viper.Viper) (chain.ResolvedChains, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)
	experimental := make(map[uint64]bool)
	for _, c := range SupportedChains {
		name, err := settings.GetChainNameByChainSelector(c.Selector)
		if err != nil {
			ct.log.Error().Msgf("Invalid Aptos chain selector %d; skipping", c.Selector)
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, name)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			ct.log.Debug().Msgf("RPC not provided for %s; skipping", name)
			continue
		}
		ct.log.Debug().Msgf("Using RPC for %s: %s", name, chain.RedactURL(rpcURL))
		client, err := aptosfakes.NewAptosClient(rpcURL)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to build Aptos client for %s: %v", name, err))
			continue
		}
		clients[c.Selector] = client
		if strings.TrimSpace(c.Forwarder) != "" {
			forwarders[c.Selector] = c.Forwarder
		}
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
			return chain.ResolvedChains{}, fmt.Errorf("experimental aptos chain %d missing rpc-url", ec.ChainSelector)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental aptos chain %d missing forwarder", ec.ChainSelector)
		}
		if _, exists := clients[ec.ChainSelector]; exists {
			if forwarders[ec.ChainSelector] != ec.Forwarder {
				ui.Warning(fmt.Sprintf("Warning: experimental aptos chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n",
					ec.ChainSelector, forwarders[ec.ChainSelector], ec.Forwarder))
				forwarders[ec.ChainSelector] = ec.Forwarder
			}
			continue
		}
		ct.log.Debug().Msgf("Using RPC for experimental aptos chain %d: %s", ec.ChainSelector, chain.RedactURL(ec.RPCURL))
		client, err := aptosfakes.NewAptosClient(ec.RPCURL)
		if err != nil {
			return chain.ResolvedChains{}, fmt.Errorf("failed to create aptos client for experimental chain %d: %w", ec.ChainSelector, err)
		}
		clients[ec.ChainSelector] = client
		forwarders[ec.ChainSelector] = ec.Forwarder
		experimental[ec.ChainSelector] = true
		ui.Dim(fmt.Sprintf("Added experimental aptos chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	return chain.ResolvedChains{Clients: clients, Forwarders: forwarders, ExperimentalSelectors: experimental}, nil
}

func (ct *AptosChainType) ResolveKey(s *settings.Settings, broadcast bool) (interface{}, error) {
	seed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s.User.PrivateKey(settings.Aptos))), "0x")
	bytes, err := hex.DecodeString(seed)
	if err != nil || len(bytes) != 32 {
		if broadcast {
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key, required to broadcast. Please check CRE_APTOS_PRIVATE_KEY in your .env file or system environment: %w", err)
			}
			return nil, fmt.Errorf("CRE_APTOS_PRIVATE_KEY must be 32 hex bytes (64 chars); got len=%d", len(bytes))
		}
		bytes, _ = hex.DecodeString(defaultSentinelAptosSeed)
		ui.Warning("Using default Aptos private key for dry-run simulation. Set CRE_APTOS_PRIVATE_KEY to broadcast.")
	}
	sentinel, _ := hex.DecodeString(defaultSentinelAptosSeed)
	if broadcast && hex.EncodeToString(bytes) == hex.EncodeToString(sentinel) {
		return nil, fmt.Errorf("CRE_APTOS_PRIVATE_KEY must not be the sentinel seed under --broadcast")
	}
	k := &crypto.Ed25519PrivateKey{}
	if err := k.FromBytes(bytes); err != nil {
		return nil, fmt.Errorf("build Ed25519 key: %w", err)
	}
	return k, nil
}

func (ct *AptosChainType) ResolveTriggerData(_ context.Context, _ uint64, _ chain.TriggerParams) (interface{}, error) {
	return nil, fmt.Errorf("aptos: no trigger surface")
}

func (ct *AptosChainType) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) ([]services.Service, error) {
	typedClients := make(map[uint64]aptosfakes.AptosClient, len(cfg.Clients))
	for sel, c := range cfg.Clients {
		ac, ok := c.(aptosfakes.AptosClient)
		if !ok {
			return nil, fmt.Errorf("aptos: client for selector %d is not aptosfakes.AptosClient (got %T)", sel, c)
		}
		typedClients[sel] = ac
	}
	var pk *crypto.Ed25519PrivateKey
	if cfg.PrivateKey != nil {
		var ok bool
		pk, ok = cfg.PrivateKey.(*crypto.Ed25519PrivateKey)
		if !ok {
			return nil, fmt.Errorf("aptos: private key is not *crypto.Ed25519PrivateKey (got %T)", cfg.PrivateKey)
		}
	}
	var lim chain.Limits
	if cfg.Limits != nil {
		lim = ExtractLimits(cfg.Limits)
	}
	caps, err := NewAptosChainCapabilities(ctx, cfg.Logger, cfg.Registry, typedClients, cfg.Forwarders, pk, !cfg.Broadcast, lim)
	if err != nil {
		return nil, err
	}
	if err := caps.Start(ctx); err != nil {
		return nil, fmt.Errorf("aptos: failed to start: %w", err)
	}
	ct.aptosChains = caps
	out := make([]services.Service, 0, len(caps.AptosChains))
	for _, fc := range caps.AptosChains {
		out = append(out, fc)
	}
	return out, nil
}

func (ct *AptosChainType) ExecuteTrigger(_ context.Context, _ uint64, _ string, _ interface{}) error {
	return fmt.Errorf("aptos: no trigger surface")
}

func (ct *AptosChainType) Supports(selector uint64) bool {
	if ct.aptosChains == nil {
		return false
	}
	return ct.aptosChains.AptosChains[selector] != nil
}

func (ct *AptosChainType) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	const prefix = "aptos:ChainSelector:"
	const suffix = "@1.0.0"
	if !strings.HasPrefix(triggerID, prefix) || !strings.HasSuffix(triggerID, suffix) {
		return 0, false
	}
	mid := triggerID[len(prefix) : len(triggerID)-len(suffix)]
	sel, err := strconv.ParseUint(mid, 10, 64)
	if err != nil {
		return 0, false
	}
	return sel, true
}

func (ct *AptosChainType) RunHealthCheck(resolved chain.ResolvedChains) error {
	return RunRPCHealthCheck(resolved.Clients, resolved.ExperimentalSelectors)
}

func (ct *AptosChainType) CollectCLIInputs(_ *viper.Viper) map[string]string {
	return map[string]string{}
}
