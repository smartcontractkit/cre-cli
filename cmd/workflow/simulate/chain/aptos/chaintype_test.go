package aptos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestResolveKey_SentinelUnderBroadcastFails(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{AptosPrivateKey: "0000000000000000000000000000000000000000000000000000000000000001"}}
	_, err := ct.ResolveKey(s, true)
	require.Error(t, err)
}

func TestResolveKey_UnparseableUnderBroadcastFails(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{AptosPrivateKey: "not-hex"}}
	_, err := ct.ResolveKey(s, true)
	require.Error(t, err)
}

func TestResolveKey_UnparseableNonBroadcastFallsBackToSentinel(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{AptosPrivateKey: ""}}
	k, err := ct.ResolveKey(s, false)
	require.NoError(t, err)
	assert.NotNil(t, k)
}

func TestResolveKey_ValidKeyBroadcast(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{AptosPrivateKey: "1111111111111111111111111111111111111111111111111111111111111111"}}
	k, err := ct.ResolveKey(s, true)
	require.NoError(t, err)
	assert.NotNil(t, k)
}

func TestParseTriggerChainSelector(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	sel, ok := ct.ParseTriggerChainSelector("aptos:ChainSelector:4741433654826277614@1.0.0")
	require.True(t, ok)
	assert.Equal(t, uint64(4741433654826277614), sel)
	_, ok = ct.ParseTriggerChainSelector("evm:ChainSelector:1@1.0.0")
	assert.False(t, ok)
}

func TestSupports_False(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	assert.False(t, ct.Supports(1))
}
