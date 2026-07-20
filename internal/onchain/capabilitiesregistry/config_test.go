package capabilitiesregistry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
)

func TestParseCapabilityConfiguration(t *testing.T) {
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

	cfg, err := ParseCapabilityConfiguration(raw)
	require.NoError(t, err)
	require.NotNil(t, cfg.DefaultConfig)

	var out map[string]any
	require.NoError(t, cfg.DefaultConfig.UnwrapTo(&out))
	require.Equal(t, "abc123", out["VaultPublicKey"])
	require.EqualValues(t, 2, out["Threshold"])
}

func TestParseCapabilityConfiguration_InvalidProto(t *testing.T) {
	t.Parallel()

	_, err := ParseCapabilityConfiguration([]byte("not-proto"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unmarshal capability config")
}
