package evm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const defaultSentinelPrivateKey = "0000000000000000000000000000000000000000000000000000000000000001"

var sentinelKeyBytes = common.FromHex(defaultSentinelPrivateKey)

func init() {
	chain.Register(string(corekeys.EVM), func(lggr *zerolog.Logger) chain.ChainType {
		return &EVMChainType{log: lggr}
	}, []chain.CLIFlagDef{
		{Name: TriggerInputTxHash, Description: "EVM trigger transaction hash (0x...)", FlagType: chain.CLIFlagString},
		{Name: TriggerInputEventIndex, Description: "EVM trigger log index (0-based)", DefaultValue: "-1", FlagType: chain.CLIFlagInt},
	})
}

// EVMChainType implements chain.ChainType for EVM-based blockchains.
type EVMChainType struct {
	log       *zerolog.Logger
	evmChains *EVMChainCapabilities
}

var _ chain.ChainType = (*EVMChainType)(nil)

func (ct *EVMChainType) Name() string { return "evm" }

func (ct *EVMChainType) SupportedChains() []chain.ChainConfig {
	return SupportedChains
}

func (ct *EVMChainType) ResolveClients(v *viper.Viper) (chain.ResolvedChains, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)
	experimental := make(map[uint64]bool)

	// build clients for each supported chain from settings, skip if rpc is empty
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
		return chain.ResolvedChains{}, fmt.Errorf("failed to load experimental chains config: %w", err)
	}

	for _, ec := range expChains {
		// Empty chain-type falls back to this chain type
		if ec.ChainType != "" && !strings.EqualFold(ec.ChainType, ct.Name()) {
			continue
		}
		if ec.ChainSelector == 0 {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain missing chain-selector")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain %d missing rpc-url", ec.ChainSelector)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain %d missing forwarder", ec.ChainSelector)
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
			return chain.ResolvedChains{}, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainSelector, err)
		}
		clients[ec.ChainSelector] = c
		forwarders[ec.ChainSelector] = ec.Forwarder
		experimental[ec.ChainSelector] = true
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	return chain.ResolvedChains{
		Clients:               clients,
		Forwarders:            forwarders,
		ExperimentalSelectors: experimental,
	}, nil
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

	var evmLimits chain.Limits
	if cfg.Limits != nil {
		evmLimits = ExtractLimits(cfg.Limits)
	}

	evmCaps, err := NewEVMChainCapabilities(
		ctx, cfg.Logger, cfg.Registry,
		ethClients, cfg.Forwarders, pk,
		dryRun, evmLimits,
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

// Supports reports whether an EVM chain capability is live for the selector.
func (ct *EVMChainType) Supports(selector uint64) bool {
	if ct.evmChains == nil {
		return false
	}
	return ct.evmChains.EVMChains[selector] != nil
}

func (ct *EVMChainType) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	return chain.ParseTriggerChainSelector(ct.Name(), triggerID)
}

func (ct *EVMChainType) RunHealthCheck(resolved chain.ResolvedChains) error {
	return RunRPCHealthCheck(resolved.Clients, resolved.ExperimentalSelectors)
}

// ResolveKey parses the user's ECDSA private key from settings. When broadcast
// is true, an invalid or default-sentinel key is a hard error. Otherwise a
// sentinel key is used with a warning so non-broadcast simulations can run.
func (ct *EVMChainType) ResolveKey(creSettings *settings.Settings, broadcast bool) (interface{}, error) {
	pk, err := crypto.HexToECDSA(creSettings.User.PrivateKey(settings.EVM))
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
	if broadcast && bytes.Equal(crypto.FromECDSA(pk), sentinelKeyBytes) {
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
