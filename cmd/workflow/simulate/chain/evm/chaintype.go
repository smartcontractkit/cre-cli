package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const defaultSentinelPrivateKey = "0000000000000000000000000000000000000000000000000000000000000001"

func init() {
	chain.Register("evm", func(lggr *zerolog.Logger) chain.ChainType {
		return &EVMChainType{log: lggr}
	}, []chain.CLIFlagDef{
		{Name: TriggerInputTxHash, Description: "EVM trigger transaction hash (0x...)", FlagType: chain.CLIFlagString},
		{Name: TriggerInputEventIndex, Description: "EVM trigger log index (0-based)", DefaultValue: "-1", FlagType: chain.CLIFlagInt},
	})
}

// EVMChainType implements chain.ChainType for EVM-based blockchains.
type EVMChainType struct {
	log                   *zerolog.Logger
	evmChains             *EVMChainCapabilities
	experimentalSelectors map[uint64]bool
}

var _ chain.ChainType = (*EVMChainType)(nil)

func (ct *EVMChainType) Name() string { return "evm" }

func (ct *EVMChainType) SupportedChains() []chain.ChainConfig {
	return SupportedChains
}

func (ct *EVMChainType) ResolveClients(v *viper.Viper) (map[uint64]chain.ChainClient, map[uint64]string, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)
	experimental := make(map[uint64]bool)

	// Resolve supported chains
	for _, ch := range SupportedChains {
		chainName, err := settings.GetChainNameByChainSelector(ch.Selector)
		if err != nil {
			ct.log.Error().Msgf("Invalid chain selector for supported EVM chains %d; skipping", ch.Selector)
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, chainName)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			ct.log.Debug().Msgf("RPC not provided for %s; skipping", chainName)
			continue
		}
		ct.log.Debug().Msgf("Using RPC for %s: %s", chainName, chain.RedactURL(rpcURL))

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
			if common.HexToAddress(forwarders[ec.ChainSelector]) != common.HexToAddress(ec.Forwarder) {
				ui.Warning(fmt.Sprintf("Warning: experimental chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n",
					ec.ChainSelector, forwarders[ec.ChainSelector], ec.Forwarder))
				forwarders[ec.ChainSelector] = ec.Forwarder
			} else {
				ct.log.Debug().Uint64("chain-selector", ec.ChainSelector).Msg("Experimental chain matches supported chain config")
			}
			continue
		}

		ct.log.Debug().Msgf("Using RPC for experimental chain %d: %s", ec.ChainSelector, chain.RedactURL(ec.RPCURL))
		c, err := ethclient.Dial(ec.RPCURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainSelector, err)
		}
		clients[ec.ChainSelector] = c
		forwarders[ec.ChainSelector] = ec.Forwarder
		experimental[ec.ChainSelector] = true
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	ct.experimentalSelectors = experimental
	return clients, forwarders, nil
}

func (ct *EVMChainType) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) ([]services.Service, error) {
	// Convert generic ChainClient map to typed *ethclient.Client map
	ethClients := make(map[uint64]*ethclient.Client)
	for sel, c := range cfg.Clients {
		ec, ok := c.(*ethclient.Client)
		if !ok {
			return nil, fmt.Errorf("EVM: client for selector %d is not *ethclient.Client", sel)
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
			return nil, fmt.Errorf("EVM: private key is not *ecdsa.PrivateKey")
		}
	}

	dryRun := !cfg.Broadcast

	// cfg.Limits already satisfies EVMChainLimits via the chain.Limits interface
	// contract; no type assertion needed.
	evmCaps, err := NewEVMChainCapabilities(
		ctx, cfg.Logger, cfg.Registry,
		ethClients, evmForwarders, pk,
		dryRun, cfg.Limits,
	)
	if err != nil {
		return nil, err
	}

	// Start the EVM chains so they begin listening for triggers
	if err := evmCaps.Start(ctx); err != nil {
		return nil, fmt.Errorf("EVM: failed to start chain capabilities: %w", err)
	}

	ct.evmChains = evmCaps

	srvcs := make([]services.Service, 0, len(evmCaps.EVMChains))
	for _, evm := range evmCaps.EVMChains {
		srvcs = append(srvcs, evm)
	}
	return srvcs, nil
}

func (ct *EVMChainType) ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error {
	if ct.evmChains == nil {
		return fmt.Errorf("EVM: capabilities not registered")
	}
	evmChain := ct.evmChains.EVMChains[selector]
	if evmChain == nil {
		return fmt.Errorf("no EVM chain initialized for selector %d", selector)
	}
	log, ok := triggerData.(*evmpb.Log)
	if !ok {
		return fmt.Errorf("EVM: trigger data is not *evm.Log")
	}
	return evmChain.ManualTrigger(ctx, registrationID, log)
}

// HasSelector reports whether an EVM chain capability has been initialised
// for the given selector. Callers use this at trigger-setup time to avoid
// building a TriggerFunc for a selector the chain type cannot dispatch against.
func (ct *EVMChainType) HasSelector(selector uint64) bool {
	if ct.evmChains == nil {
		return false
	}
	return ct.evmChains.EVMChains[selector] != nil
}

func (ct *EVMChainType) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	return ParseTriggerChainSelector(triggerID)
}

func (ct *EVMChainType) RunHealthCheck(clients map[uint64]chain.ChainClient) error {
	return RunRPCHealthCheck(clients, ct.experimentalSelectors)
}

// ResolveKey parses the user's ECDSA private key from settings. When broadcast
// is true, an invalid or default-sentinel key is a hard error. Otherwise a
// sentinel key is used with a warning so non-broadcast simulations can run.
func (ct *EVMChainType) ResolveKey(creSettings *settings.Settings, broadcast bool) (interface{}, error) {
	pk, err := crypto.HexToECDSA(creSettings.User.EthPrivateKey)
	if err != nil {
		if broadcast {
			return nil, fmt.Errorf(
				"failed to parse private key, required to broadcast. Please check CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
		}
		pk, err = crypto.HexToECDSA(defaultSentinelPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse default private key. Please set CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
		}
		ui.Warning("Using default private key for chain write simulation. To use your own key, set CRE_ETH_PRIVATE_KEY in your .env file or system environment.")
	}
	if broadcast && pk.D.Cmp(big.NewInt(1)) == 0 {
		return nil, fmt.Errorf("you must configure a valid private key to perform on-chain writes. Please set your private key in the .env file before using the --broadcast flag")
	}
	return pk, nil
}

// CLI input keys consumed from chain.TriggerParams.ChainTypeInputs.
const (
	TriggerInputTxHash     = "evm-tx-hash"
	TriggerInputEventIndex = "evm-event-index"
)

func (ct *EVMChainType) CollectCLIInputs(v *viper.Viper) map[string]string {
	inputs := map[string]string{}
	if txHash := strings.TrimSpace(v.GetString(TriggerInputTxHash)); txHash != "" {
		inputs[TriggerInputTxHash] = txHash
	}
	if idx := v.GetInt(TriggerInputEventIndex); idx >= 0 {
		inputs[TriggerInputEventIndex] = strconv.Itoa(idx)
	}
	return inputs
}

// ResolveTriggerData fetches the EVM log payload for the given selector from
// CLI-supplied or interactively-prompted inputs.
func (ct *EVMChainType) ResolveTriggerData(ctx context.Context, selector uint64, params chain.TriggerParams) (interface{}, error) {
	clientIface, ok := params.Clients[selector]
	if !ok {
		return nil, fmt.Errorf("no RPC configured for chain selector %d", selector)
	}
	client, ok := clientIface.(*ethclient.Client)
	if !ok {
		return nil, fmt.Errorf("invalid client type for EVM chain selector %d", selector)
	}

	if params.Interactive {
		return GetEVMTriggerLog(ctx, client)
	}

	txHash := strings.TrimSpace(params.ChainTypeInputs[TriggerInputTxHash])
	eventIndexStr := strings.TrimSpace(params.ChainTypeInputs[TriggerInputEventIndex])
	if txHash == "" || eventIndexStr == "" {
		return nil, fmt.Errorf("--evm-tx-hash and --evm-event-index are required for EVM triggers in non-interactive mode")
	}
	eventIndex, err := strconv.ParseUint(eventIndexStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid --evm-event-index %q: %w", eventIndexStr, err)
	}
	return GetEVMTriggerLogFromValues(ctx, client, txHash, eventIndex)
}
