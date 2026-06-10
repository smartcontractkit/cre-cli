package capabilitiesregistry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
)

func TestParseVaultCapabilityConfig(t *testing.T) {
	t.Parallel()

	valueMap, err := values.WrapMap(map[string]any{
		"VaultPublicKey": "abc123",
		"Threshold":      2,
	})
	require.NoError(t, err)

	raw, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	cfg, err := ParseVaultCapabilityConfig(raw)
	require.NoError(t, err)
	require.Equal(t, "abc123", cfg.VaultPublicKey)
	require.Equal(t, 2, cfg.Threshold)
}

func TestParseVaultCapabilityConfig_MissingPublicKey(t *testing.T) {
	t.Parallel()

	valueMap, err := values.WrapMap(map[string]any{"Threshold": 1})
	require.NoError(t, err)

	raw, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	_, err = ParseVaultCapabilityConfig(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "VaultPublicKey")
}

func TestParseVaultCapabilityConfig_InvalidThreshold(t *testing.T) {
	t.Parallel()

	valueMap, err := values.WrapMap(map[string]any{
		"VaultPublicKey": "abc123",
		"Threshold":      0,
	})
	require.NoError(t, err)

	raw, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	_, err = ParseVaultCapabilityConfig(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Threshold")
}
