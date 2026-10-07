package stellar

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	capreg "github.com/smartcontractkit/chainlink-common/pkg/capabilities/registry"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// A syntactically valid contract address for forwarder-override tests.
const testForwarder = "CDNKVWAPQZWVA2FLT3H5IQZ7N5WLYEDKUNRZ33E4KY7TI5KOBU6MTAVW"

func newStellarChainType() *StellarChainType {
	lg := zerolog.Nop()
	return &StellarChainType{log: &lg}
}

func newViper(t *testing.T, target map[string]any) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "t")
	for k, val := range target {
		v.Set("t."+k, val)
	}
	return v
}

func TestStellarChainType_Registered(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	chain.Build(&lg)
	ct, err := chain.Get("stellar")
	require.NoError(t, err)
	assert.Equal(t, "stellar", ct.Name())
}

func TestSupportedChains(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		if c.Forwarder != "" {
			assert.True(t, strkey.IsValidContractAddress(c.Forwarder), "forwarder for %d", c.Selector)
		}
		assert.NotEmpty(t, networkPassphrase(c.Selector))
		_, err := settings.GetChainNameByChainSelector(c.Selector)
		require.NoError(t, err)
	}
}

func TestResolveClients(t *testing.T) {
	t.Parallel()
	v := newViper(t, map[string]any{
		settings.RpcsSettingName: []map[string]any{
			{"chain-name": "stellar-testnet", "url": "https://soroban-testnet.example"},
			{"chain-name": "ethereum-testnet-sepolia", "url": "https://sepolia.example"},
		},
	})
	resolved, err := newStellarChainType().ResolveClients(v)
	require.NoError(t, err)
	sel := chainselectors.STELLAR_TESTNET.Selector
	require.Len(t, resolved.Clients, 1)
	assert.Contains(t, resolved.Clients, sel)
	assert.Equal(t, testnetMockForwarder, resolved.Forwarders[sel])
}

func TestResolveClients_ExperimentalForwarderOverride(t *testing.T) {
	t.Parallel()
	const override = "CC3EIGKQNIXPRYMQS5YVQLM62XZZX3JBZETSOQVA2S5SKNTGFTSDUCSK"
	sel := chainselectors.STELLAR_TESTNET.Selector
	v := newViper(t, map[string]any{
		settings.RpcsSettingName: []map[string]any{
			{"chain-name": "stellar-testnet", "url": "https://soroban-testnet.example"},
		},
		settings.ExperimentalChainsSettingName: []map[string]any{
			{"chain-type": "stellar", "chain-selector": sel, "forwarder": override},
			// EVM entries (explicit or default chain-type) are ignored.
			{"chain-selector": 123, "rpc-url": "https://x", "forwarder": "0x1"},
		},
	})
	resolved, err := newStellarChainType().ResolveClients(v)
	require.NoError(t, err)
	assert.Equal(t, override, resolved.Forwarders[sel])
	assert.Len(t, resolved.Clients, 1)
	assert.Empty(t, resolved.ExperimentalSelectors)
}

func TestResolveClients_ExperimentalChain(t *testing.T) {
	t.Parallel()
	sel := chainselectors.STELLAR_LOCALNET.Selector
	v := newViper(t, map[string]any{
		settings.ExperimentalChainsSettingName: []map[string]any{
			{"chain-type": "stellar", "chain-selector": sel, "rpc-url": "http://localhost:8000/rpc", "forwarder": testForwarder},
		},
	})
	resolved, err := newStellarChainType().ResolveClients(v)
	require.NoError(t, err)
	assert.Contains(t, resolved.Clients, sel)
	assert.True(t, resolved.ExperimentalSelectors[sel])
}

func TestResolveClients_ExperimentalInvalidForwarder(t *testing.T) {
	t.Parallel()
	v := newViper(t, map[string]any{
		settings.ExperimentalChainsSettingName: []map[string]any{
			{"chain-type": "stellar", "chain-selector": 1, "rpc-url": "http://x", "forwarder": "0xabc"},
		},
	})
	_, err := newStellarChainType().ResolveClients(v)
	require.ErrorContains(t, err, "invalid forwarder")
}

func TestStellarChainType_ResolveKey(t *testing.T) {
	t.Parallel()
	ct := newStellarChainType()
	withKey := func(k string) *settings.Settings {
		return &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Stellar.Name: k}}}
	}

	key, err := ct.ResolveKey(withKey(""), false)
	require.NoError(t, err, "dry-run needs no key")
	assert.Nil(t, key)

	_, err = ct.ResolveKey(withKey(""), true)
	require.ErrorContains(t, err, "CRE_STELLAR_PRIVATE_KEY is required for --broadcast")

	_, err = ct.ResolveKey(withKey("not-a-seed"), false)
	require.ErrorContains(t, err, "secret seed")

	kp := keypair.MustRandom()
	key, err = ct.ResolveKey(withKey(kp.Seed()), true)
	require.NoError(t, err)
	require.IsType(t, &keypair.Full{}, key)
	assert.Equal(t, kp.Address(), key.(*keypair.Full).Address())
}

func TestStellarChainType_NoTriggers(t *testing.T) {
	t.Parallel()
	ct := newStellarChainType()
	_, err := ct.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{})
	require.Error(t, err)
	require.Error(t, ct.ExecuteTrigger(context.Background(), 1, "id", nil))
	assert.False(t, ct.Supports(chainselectors.STELLAR_TESTNET.Selector))

	sel, ok := ct.ParseTriggerChainSelector("stellar:ChainSelector:4894814558906953166@1.0.0")
	assert.True(t, ok)
	assert.Equal(t, chainselectors.STELLAR_TESTNET.Selector, sel)
	_, ok = ct.ParseTriggerChainSelector("evm:ChainSelector:1@1.0.0")
	assert.False(t, ok)
}

// registerCfg builds a CapabilityConfig for one stellar-testnet client. The
// client is never dialled: the testnet passphrase comes from chain-selectors.
func registerCfg(t *testing.T, forwarder string, broadcast bool, key interface{}) (chain.CapabilityConfig, *capreg.Registry) {
	t.Helper()
	sel := chainselectors.STELLAR_TESTNET.Selector
	reg := capreg.NewRegistry(logger.Test(t))
	forwarders := map[uint64]string{}
	if forwarder != "" {
		forwarders[sel] = forwarder
	}
	return chain.CapabilityConfig{
		Registry:   reg,
		Clients:    map[uint64]chain.ChainClient{sel: rpcclient.NewClient("http://localhost:1", http.DefaultClient)},
		Forwarders: forwarders,
		PrivateKey: key,
		Broadcast:  broadcast,
		Logger:     logger.Test(t),
	}, reg
}

func stellarCapabilityID(sel uint64) string {
	return fmt.Sprintf("stellar:ChainSelector:%d@1.0.0", sel)
}

func TestStellarChainType_RegisterCapabilities_DryRunWithoutKey(t *testing.T) {
	t.Parallel()
	sel := chainselectors.STELLAR_TESTNET.Selector
	ct := newStellarChainType()
	cfg, reg := registerCfg(t, testForwarder, false, nil)

	srvcs, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.NoError(t, err, "dry-run must not require a transmitter key")
	require.Len(t, srvcs, 1)
	assert.True(t, ct.Supports(sel))

	_, err = reg.Get(context.Background(), stellarCapabilityID(sel))
	require.NoError(t, err, "capability must be registered")
}

func TestStellarChainType_RegisterCapabilities_BroadcastWithoutKeyFails(t *testing.T) {
	t.Parallel()
	ct := newStellarChainType()
	cfg, reg := registerCfg(t, testForwarder, true, nil)

	// Broadcast maps to DryRun=false, which the fake rejects without a key. If
	// the mapping were inverted this would succeed and simulations would submit.
	_, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.ErrorContains(t, err, "transmitter key is required")

	caps, err := reg.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, caps)
}

func TestStellarChainType_RegisterCapabilities_BroadcastWithKey(t *testing.T) {
	t.Parallel()
	sel := chainselectors.STELLAR_TESTNET.Selector
	ct := newStellarChainType()
	cfg, reg := registerCfg(t, testForwarder, true, keypair.MustRandom())

	srvcs, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, srvcs, 1)
	assert.True(t, ct.Supports(sel))
	_, err = reg.Get(context.Background(), stellarCapabilityID(sel))
	require.NoError(t, err)
}

func TestStellarChainType_RegisterCapabilities_NoForwarderSkipsChain(t *testing.T) {
	t.Parallel()
	ct := newStellarChainType()
	cfg, reg := registerCfg(t, "", false, nil)

	srvcs, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.NoError(t, err)
	assert.Empty(t, srvcs)
	assert.False(t, ct.Supports(chainselectors.STELLAR_TESTNET.Selector))

	caps, err := reg.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, caps)
}

func TestStellarChainType_RegisterCapabilities_InvalidInputs(t *testing.T) {
	t.Parallel()

	t.Run("wrong key type", func(t *testing.T) {
		t.Parallel()
		cfg, _ := registerCfg(t, testForwarder, true, "not-a-keypair")
		_, err := newStellarChainType().RegisterCapabilities(context.Background(), cfg)
		require.ErrorContains(t, err, "private key is not *keypair.Full")
	})

	t.Run("wrong client type", func(t *testing.T) {
		t.Parallel()
		cfg, _ := registerCfg(t, testForwarder, false, nil)
		cfg.Clients = map[uint64]chain.ChainClient{chainselectors.STELLAR_TESTNET.Selector: "not-a-client"}
		_, err := newStellarChainType().RegisterCapabilities(context.Background(), cfg)
		require.ErrorContains(t, err, "is not *rpcclient.Client")
	})

	t.Run("invalid forwarder", func(t *testing.T) {
		t.Parallel()
		cfg, _ := registerCfg(t, "not-a-contract", false, nil)
		_, err := newStellarChainType().RegisterCapabilities(context.Background(), cfg)
		require.ErrorContains(t, err, "forwarder")
	})
}
