package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
)

func TestUnmarshalVaultResponsePayload_DiscardUnknownFields(t *testing.T) {
	payload := []byte(`{"requestId":"req-123","responses":[{"id":{"key":"apiKey","owner":"0xabc","namespace":"main"},"success":true}]}`)

	var p vault.CreateSecretsResponse
	require.NoError(t, unmarshalVaultResponsePayload(payload, &p))
	require.Len(t, p.GetResponses(), 1)
	require.True(t, p.GetResponses()[0].GetSuccess())
	require.Equal(t, "apiKey", p.GetResponses()[0].GetId().GetKey())
}
