package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"

	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func init() {
	chain.Register(&EVMFamily{})
}

// EVMFamily implements chain.ChainFamily for EVM-based blockchains.
type EVMFamily struct {
	evmChains *EVMChainCapabilities
}

var _ chain.ChainFamily = (*EVMFamily)(nil)

func (f *EVMFamily) Name() string { return "evm" }

func (f *EVMFamily) SupportedChains() []chain.ChainConfig {
	return SupportedChains
}

func (f *EVMFamily) ResolveClients(v *viper.Viper) (map[uint64]chain.ChainClient, map[uint64]string, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)

	// Resolve supported chains
	for _, ch := range SupportedChains {
		chainName, err := settings.GetChainNameByChainSelector(ch.Selector)
		if err != nil {
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, chainName)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			continue
		}
		c, err := ethclient.Dial(rpcURL)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to create eth client for %s: %v", chainName, err))
			continue
		}
		clients[ch.Selector] = c
		if strings.TrimSpace(ch.Forwarder) != "" {
			forwarders[ch.Selector] = ch.Forwarder
		}
	}

	// Resolve experimental chains
	expChains, err := settings.GetExperimentalChains(v)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load experimental chains config: %w", err)
	}

	for _, ec := range expChains {
		if ec.ChainSelector == 0 {
			return nil, nil, fmt.Errorf("experimental chain missing chain-selector")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return nil, nil, fmt.Errorf("experimental chain %d missing rpc-url", ec.ChainSelector)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return nil, nil, fmt.Errorf("experimental chain %d missing forwarder", ec.ChainSelector)
		}

		// For duplicate selectors, keep the supported client and only
		// override the forwarder.
		if _, exists := clients[ec.ChainSelector]; exists {
			if !strings.EqualFold(forwarders[ec.ChainSelector], ec.Forwarder) {
				ui.Warning(fmt.Sprintf("Warning: experimental chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n",
					ec.ChainSelector, forwarders[ec.ChainSelector], ec.Forwarder))
				forwarders[ec.ChainSelector] = ec.Forwarder
			}
			continue
		}

		c, err := ethclient.Dial(ec.RPCURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainSelector, err)
		}
		clients[ec.ChainSelector] = c
		forwarders[ec.ChainSelector] = ec.Forwarder
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	return clients, forwarders, nil
}

func (f *EVMFamily) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) error {
	// Convert generic ChainClient map to typed *ethclient.Client map
	ethClients := make(map[uint64]*ethclient.Client)
	for sel, c := range cfg.Clients {
		ec, ok := c.(*ethclient.Client)
		if !ok {
			return fmt.Errorf("EVM family: client for selector %d is not *ethclient.Client", sel)
		}
		ethClients[sel] = ec
	}

	// Convert string forwarders to common.Address
	evmForwarders := make(map[uint64]common.Address)
	for sel, fwd := range cfg.Forwarders {
		evmForwarders[sel] = common.HexToAddress(fwd)
	}

	// Type-assert the private key
	var pk *ecdsa.PrivateKey
	if cfg.PrivateKey != nil {
		var ok bool
		pk, ok = cfg.PrivateKey.(*ecdsa.PrivateKey)
		if !ok {
			return fmt.Errorf("EVM family: private key is not *ecdsa.PrivateKey")
		}
	}

	// Type-assert the limits (SimulationLimits satisfies EVMChainLimits implicitly)
	var evmLimits EVMChainLimits
	if cfg.Limits != nil {
		var ok bool
		evmLimits, ok = cfg.Limits.(EVMChainLimits)
		if !ok {
			return fmt.Errorf("EVM family: limits does not satisfy EVMChainLimits interface")
		}
	}

	dryRun := !cfg.Broadcast

	evmCaps, err := NewEVMChainCapabilities(
		ctx, cfg.Logger, cfg.Registry,
		ethClients, evmForwarders, pk,
		dryRun, evmLimits,
	)
	if err != nil {
		return err
	}

	f.evmChains = evmCaps
	return nil
}

func (f *EVMFamily) ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error {
	if f.evmChains == nil {
		return fmt.Errorf("EVM family: capabilities not registered")
	}
	evmChain := f.evmChains.EVMChains[selector]
	if evmChain == nil {
		return fmt.Errorf("no EVM chain initialized for selector %d", selector)
	}
	log, ok := triggerData.(*evmpb.Log)
	if !ok {
		return fmt.Errorf("EVM family: trigger data is not *evm.Log")
	}
	return evmChain.ManualTrigger(ctx, registrationID, log)
}

func (f *EVMFamily) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	return ParseTriggerChainSelector(triggerID)
}

func (f *EVMFamily) RunHealthCheck(clients map[uint64]chain.ChainClient) error {
	return RunRPCHealthCheck(clients, nil)
}
