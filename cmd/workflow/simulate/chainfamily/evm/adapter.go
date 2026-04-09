package evm

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chainfamily"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func init() {
	chainfamily.Register(&EVMAdapter{})
}

// EVMAdapter implements chainfamily.Adapter for EVM-compatible chains.
type EVMAdapter struct{}

func (a *EVMAdapter) Family() string { return "evm" }

func (a *EVMAdapter) SupportedChains() []chainfamily.SupportedChain {
	return SupportedChains
}

func (a *EVMAdapter) AddFlags(cmd *cobra.Command) {
	cmd.Flags().String("evm-tx-hash", "", "EVM trigger transaction hash (0x...)")
	cmd.Flags().Int("evm-event-index", -1, "EVM trigger log index (0-based)")
	cmd.Flags().Bool("broadcast", false, "Broadcast transactions to the EVM (default: false)")
}

func (a *EVMAdapter) Setup(ctx context.Context, cfg chainfamily.SetupConfig) (chainfamily.ChainRuntime, error) {
	if cfg.Logger == nil {
		cfg.Logger = logger.Nop()
	}
	lggr := logger.Sugared(cfg.Logger)

	// Resolve the private key (EVM-specific credential)
	pk, dryRun, err := resolvePrivateKey(cfg)
	if err != nil {
		return nil, err
	}

	// Build clients for each supported chain
	clients := make(map[uint64]*ethclient.Client)
	for _, chain := range SupportedChains {
		chainName, err := settings.GetChainNameByChainSelector(chain.Selector)
		if err != nil {
			lggr.Debugw("Invalid chain selector for supported EVM chain; skipping", "selector", chain.Selector)
			continue
		}
		rpcURL, ok := cfg.RPCURLs[chainName]
		if !ok || strings.TrimSpace(rpcURL) == "" {
			lggr.Debugw("RPC not provided; skipping", "chain", chainName)
			continue
		}

		c, dialErr := ethclient.Dial(rpcURL)
		if dialErr != nil {
			ui.Warning(fmt.Sprintf("Failed to create eth client for %s: %v", chainName, dialErr))
			continue
		}
		clients[chain.Selector] = c
	}

	// Build forwarder map from supported chains that have clients
	forwarders := map[uint64]common.Address{}
	for _, c := range SupportedChains {
		if _, ok := clients[c.Selector]; ok && strings.TrimSpace(c.Forwarder) != "" {
			forwarders[c.Selector] = common.HexToAddress(c.Forwarder)
		}
	}

	// Add experimental chains (already filtered to EVM by the core)
	experimentalForwarders := map[uint64]common.Address{}
	for _, ec := range cfg.ExperimentalChains {
		if ec.ChainSelector == 0 {
			return nil, fmt.Errorf("experimental EVM chain missing chain-selector")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return nil, fmt.Errorf("experimental EVM chain %d missing rpc-url", ec.ChainSelector)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return nil, fmt.Errorf("experimental EVM chain %d missing forwarder", ec.ChainSelector)
		}

		// Check if chain selector already exists (supported chain)
		if _, exists := clients[ec.ChainSelector]; exists {
			var supportedForwarder string
			for _, supported := range SupportedChains {
				if supported.Selector == ec.ChainSelector {
					supportedForwarder = supported.Forwarder
					break
				}
			}

			expFwd := common.HexToAddress(ec.Forwarder)
			if supportedForwarder != "" && common.HexToAddress(supportedForwarder) == expFwd {
				lggr.Debugw("Experimental chain matches supported chain config", "chain-selector", ec.ChainSelector)
				continue
			}

			ui.Warning(fmt.Sprintf("Warning: experimental chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n",
				ec.ChainSelector, supportedForwarder, ec.Forwarder))
			experimentalForwarders[ec.ChainSelector] = expFwd
			forwarders[ec.ChainSelector] = expFwd
			continue
		}

		// Dial the RPC for new experimental chain
		c, dialErr := ethclient.Dial(ec.RPCURL)
		if dialErr != nil {
			return nil, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainSelector, dialErr)
		}
		clients[ec.ChainSelector] = c
		fwd := common.HexToAddress(ec.Forwarder)
		experimentalForwarders[ec.ChainSelector] = fwd
		forwarders[ec.ChainSelector] = fwd
		ui.Dim(fmt.Sprintf("Added experimental EVM chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	if len(clients) == 0 {
		// No EVM chains configured - return nil, nil to indicate this adapter has nothing to do
		return nil, nil
	}

	// RPC health check
	if hcErr := runRPCHealthCheck(clients, experimentalForwarders); hcErr != nil {
		ui.Warning(fmt.Sprintf("Some EVM RPCs are not functioning properly: %v", hcErr))
	}

	// Create fake EVM chains (capability registration is deferred to RegisterCapabilities)
	chains := make(map[uint64]*fakes.FakeEVMChain)
	srvcs := make([]services.Service, 0)
	for sel, client := range clients {
		fwd, ok := forwarders[sel]
		if !ok {
			lggr.Infow("Forwarder not found for chain", "selector", sel)
			continue
		}

		evm := fakes.NewFakeEvmChain(
			cfg.Logger,
			client,
			pk,
			fwd,
			sel,
			dryRun,
		)

		chains[sel] = evm
		srvcs = append(srvcs, evm)
	}

	// Start all EVM chains
	for sel, chain := range chains {
		if err := chain.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start EVM chain %d: %w", sel, err)
		}
	}

	return &evmRuntime{
		clients:              clients,
		chains:               chains,
		services:             srvcs,
		reportSizeLimitBytes: cfg.ChainWriteReportSizeLimit,
		gasLimit:             cfg.ChainWriteGasLimit,
	}, nil
}

// resolvePrivateKey resolves the EVM private key from the environment.
// Returns the key, effective dry-run flag, and any error.
func resolvePrivateKey(cfg chainfamily.SetupConfig) (*ecdsa.PrivateKey, bool, error) {
	broadcast := cfg.FlagValues("broadcast") == "true"
	dryRun := cfg.DryRun
	if broadcast {
		dryRun = false
	}

	// The private key is passed via FlagValues by the core (sourced from settings/env).
	rawKey := strings.TrimSpace(cfg.FlagValues(settings.EthPrivateKeyEnvVar))
	normalized := settings.NormalizeHexKey(rawKey)

	if normalized != "" {
		pk, err := crypto.HexToECDSA(normalized)
		if err != nil {
			if broadcast {
				return nil, false, fmt.Errorf(
					"failed to parse private key, required to broadcast. Please check CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
			}
			// Fall through to default key
		} else {
			return pk, dryRun, nil
		}
	}

	if broadcast {
		return nil, false, fmt.Errorf("you must configure a valid private key to perform on-chain writes. Please set CRE_ETH_PRIVATE_KEY")
	}

	// Use a default dummy key for dry-run simulations
	pk, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse default private key: %w", err)
	}
	ui.Warning("Using default private key for chain write simulation. To use your own key, set CRE_ETH_PRIVATE_KEY in your .env file or system environment.")
	return pk, true, nil
}

// runRPCHealthCheck runs connectivity check against every configured client.
func runRPCHealthCheck(clients map[uint64]*ethclient.Client, experimentalForwarders map[uint64]common.Address) error {
	if len(clients) == 0 {
		return fmt.Errorf("check your settings: no RPC URLs found for supported or experimental chains")
	}

	var errs []error
	for selector, c := range clients {
		if c == nil {
			errs = append(errs, fmt.Errorf("[%d] nil client", selector))
			continue
		}

		var chainLabel string
		if _, isExperimental := experimentalForwarders[selector]; isExperimental {
			chainLabel = fmt.Sprintf("experimental chain %d", selector)
		} else {
			name, err := settings.GetChainNameByChainSelector(selector)
			if err != nil {
				chainLabel = fmt.Sprintf("chain %d", selector)
			} else {
				chainLabel = name
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		chainID, err := c.ChainID(ctx)
		cancel()

		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", chainLabel, err))
			continue
		}
		if chainID == nil || chainID.Sign() <= 0 {
			errs = append(errs, fmt.Errorf("[%s] invalid RPC response: empty or zero chain ID", chainLabel))
			continue
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rpc health check failed:\n%w", errors.Join(errs...))
	}
	return nil
}
