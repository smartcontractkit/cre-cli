package stellar

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

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
