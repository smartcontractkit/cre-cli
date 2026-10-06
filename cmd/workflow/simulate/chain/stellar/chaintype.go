package stellar

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"

	stellarserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/stellar/server"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
	"github.com/smartcontractkit/chainlink-stellar/capabilities/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	crpc "github.com/smartcontractkit/cre-cli/internal/rpc"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// chainTypeName matches the capability ID prefix ("stellar:ChainSelector:<sel>@1.0.0").
const chainTypeName = "stellar"

const getNetworkTimeout = 10 * time.Second

func init() {
	chain.Register(chainTypeName, func(lggr *zerolog.Logger) chain.ChainType {
		return &StellarChainType{log: lggr}
	}, nil)
}

// StellarChainType implements chain.ChainType for Stellar (Soroban) on top of
// chainlink-stellar's FakeStellarChain. Writes go through the mock forwarder:
// simulated by default, submitted with --broadcast.
type StellarChainType struct {
	log    *zerolog.Logger
	chains map[uint64]*fakes.FakeStellarChain
}

var _ chain.ChainType = (*StellarChainType)(nil)

func (ct *StellarChainType) Name() string                         { return chainTypeName }
func (ct *StellarChainType) SupportedChains() []chain.ChainConfig { return SupportedChains }

func (ct *StellarChainType) ResolveClients(v *viper.Viper) (chain.ResolvedChains, error) {
	clients := make(map[uint64]chain.ChainClient)
	forwarders := make(map[uint64]string)
	experimental := make(map[uint64]bool)

	for _, c := range SupportedChains {
		name, err := settings.GetChainNameByChainSelector(c.Selector)
		if err != nil {
			ct.log.Error().Msgf("Invalid Stellar chain selector %d; skipping", c.Selector)
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, name)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			ct.log.Debug().Msgf("RPC not provided for %s; skipping", name)
			continue
		}
		ct.log.Debug().Msgf("Using RPC for %s: %s", name, crpc.RedactURL(rpcURL))
		clients[c.Selector] = rpcclient.NewClient(rpcURL, http.DefaultClient)
		if c.Forwarder != "" {
			forwarders[c.Selector] = c.Forwarder
		}
	}

	expChains, err := settings.GetExperimentalChains(v)
	if err != nil {
		return chain.ResolvedChains{}, fmt.Errorf("failed to load experimental chains config: %w", err)
	}
	for _, ec := range expChains {
		// Unlike EVM, an empty chain-type is not claimed: it defaults to EVM.
		if !strings.EqualFold(ec.ChainType, ct.Name()) {
			continue
		}
		if ec.ChainSelector == 0 {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain missing chain-selector")
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain %d missing forwarder", ec.ChainSelector)
		}
		if !strkey.IsValidContractAddress(ec.Forwarder) {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain %d: invalid forwarder %q: expected a Stellar contract address (C…)", ec.ChainSelector, ec.Forwarder)
		}

		// For duplicate selectors, keep the supported client and only
		// override the forwarder.
		if _, exists := clients[ec.ChainSelector]; exists {
			if forwarders[ec.ChainSelector] != ec.Forwarder {
				ui.Dim(fmt.Sprintf("Using forwarder %s for Stellar chain %d (from experimental-chains)\n", ec.Forwarder, ec.ChainSelector))
				forwarders[ec.ChainSelector] = ec.Forwarder
			}
			continue
		}

		if strings.TrimSpace(ec.RPCURL) == "" {
			return chain.ResolvedChains{}, fmt.Errorf("experimental chain %d missing rpc-url", ec.ChainSelector)
		}
		ct.log.Debug().Msgf("Using RPC for experimental chain %d: %s", ec.ChainSelector, crpc.RedactURL(ec.RPCURL))
		clients[ec.ChainSelector] = rpcclient.NewClient(ec.RPCURL, http.DefaultClient)
		forwarders[ec.ChainSelector] = ec.Forwarder
		experimental[ec.ChainSelector] = true
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-selector: %d)\n", ec.ChainSelector))
	}

	return chain.ResolvedChains{Clients: clients, Forwarders: forwarders, ExperimentalSelectors: experimental}, nil
}

// ResolveKey parses CRE_STELLAR_PRIVATE_KEY (an S… secret seed). It is
// required with --broadcast; dry-run simulation works without one.
func (ct *StellarChainType) ResolveKey(s *settings.Settings, broadcast bool) (interface{}, error) {
	raw := strings.TrimSpace(s.User.PrivateKey(settings.Stellar))
	if raw == "" {
		if broadcast {
			return nil, fmt.Errorf("%s is required for --broadcast with Stellar workflows.\n\n"+
				"Set it to the S… secret seed of a funded account on the target network, e.g.:\n\n"+
				"  stellar keys generate cre-sim --network testnet --fund\n"+
				"  stellar keys show cre-sim\n\n"+
				"and then:\n\n"+
				"  %s=<S… secret seed>", settings.Stellar.PrivateKeyEnv, settings.Stellar.PrivateKeyEnv)
		}
		return nil, nil
	}
	kp, err := keypair.ParseFull(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a Stellar secret seed (S…): %w", settings.Stellar.PrivateKeyEnv, err)
	}
	return kp, nil
}

func (ct *StellarChainType) ResolveTriggerData(context.Context, uint64, chain.TriggerParams) (interface{}, error) {
	return nil, fmt.Errorf("stellar triggers are not supported in simulation; use a cron or HTTP trigger")
}

func (ct *StellarChainType) RegisterCapabilities(ctx context.Context, cfg chain.CapabilityConfig) ([]services.Service, error) {
	var transmitter *keypair.Full
	if cfg.PrivateKey != nil {
		kp, ok := cfg.PrivateKey.(*keypair.Full)
		if !ok {
			return nil, fmt.Errorf("stellar: private key is not *keypair.Full")
		}
		transmitter = kp
	}

	var lim chain.Limits
	if cfg.Limits != nil {
		lim = ExtractLimits(cfg.Limits)
	}

	ct.chains = make(map[uint64]*fakes.FakeStellarChain, len(cfg.Clients))
	out := make([]services.Service, 0, len(cfg.Clients))
	for sel, c := range cfg.Clients {
		client, ok := c.(*rpcclient.Client)
		if !ok {
			return nil, fmt.Errorf("stellar: client for selector %d is not *rpcclient.Client", sel)
		}
		forwarder, ok := cfg.Forwarders[sel]
		if !ok || forwarder == "" {
			ui.Warning(fmt.Sprintf("No Stellar mock forwarder configured for %s; Stellar capabilities for it are disabled. "+
				"Add an experimental-chains entry with chain-type: stellar, chain-selector: %d and forwarder: <mock forwarder C… address>",
				selectorLabel(sel, nil), sel))
			continue
		}
		passphrase, err := resolvePassphrase(ctx, client, sel)
		if err != nil {
			return nil, err
		}

		dc, err := fakes.NewFakeStellarChain(cfg.Logger, client, fakes.Config{
			ChainSelector:     sel,
			NetworkPassphrase: passphrase,
			ForwarderID:       forwarder,
			Transmitter:       transmitter,
			DryRun:            !cfg.Broadcast,
		})
		if err != nil {
			return nil, fmt.Errorf("stellar: selector %d: %w", sel, err)
		}
		server := stellarserver.NewClientServer(NewLimitedStellarChain(dc, lim))
		if err := cfg.Registry.Add(ctx, server); err != nil {
			return nil, fmt.Errorf("register stellar capability for selector %d: %w", sel, err)
		}
		ct.chains[sel] = dc
		out = append(out, dc)
	}
	return out, nil
}

// resolvePassphrase prefers the chain-selectors passphrase and falls back to
// asking the RPC (needed for experimental selectors).
func resolvePassphrase(ctx context.Context, client *rpcclient.Client, sel uint64) (string, error) {
	if p := networkPassphrase(sel); p != "" {
		return p, nil
	}
	ctx, cancel := context.WithTimeout(ctx, getNetworkTimeout)
	defer cancel()
	res, err := client.GetNetwork(ctx)
	if err != nil {
		return "", fmt.Errorf("stellar: failed to fetch network passphrase for selector %d: %w", sel, err)
	}
	return res.Passphrase, nil
}

func (ct *StellarChainType) ExecuteTrigger(context.Context, uint64, string, interface{}) error {
	return fmt.Errorf("stellar triggers are not supported in simulation")
}

func (ct *StellarChainType) Supports(selector uint64) bool {
	return ct.chains[selector] != nil
}

func (ct *StellarChainType) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	return chain.ParseTriggerChainSelector(ct.Name(), triggerID)
}

func (ct *StellarChainType) RunHealthCheck(resolved chain.ResolvedChains) error {
	return RunRPCHealthCheck(resolved.Clients, resolved.ExperimentalSelectors)
}

func (ct *StellarChainType) CollectCLIInputs(*viper.Viper) map[string]string {
	return map[string]string{}
}

// ExtractLimits maps workflow limits onto Stellar writes. cresettings has no
// Stellar-specific chain-write block, so the generic report size limit applies.
func ExtractLimits(w *cresettings.Workflows) chain.Limits {
	return chain.Limits{ReportSize: int(w.ChainWrite.ReportSizeLimit.DefaultValue)}
}
