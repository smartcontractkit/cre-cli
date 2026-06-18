package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/vaultdon"
)

func attachMockVaultDONResolverWithOCRSigners(t *testing.T, h *Handler, signers []common.Address) {
	t.Helper()

	valueMap, err := values.WrapMap(map[string]any{
		"VaultPublicKey": "deadbeef",
		"Threshold":      1,
	})
	require.NoError(t, err)
	cfgBytes, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	nodes := make([]capreg.INodeInfoProviderNodeInfo, len(signers))
	for i, addr := range signers {
		p2pID := [32]byte{byte(i + 1)}
		copy(nodes[i].Signer[:], addr.Bytes())
		nodes[i].P2pId = p2pID
	}

	reader := &mockCapRegReader{
		donIDs: []*big.Int{big.NewInt(1)},
		dons: map[uint32]capreg.CapabilitiesRegistryDONInfo{
			1: {
				Id: 1,
				F:  1,
				NodeP2PIds: func() [][32]byte {
					ids := make([][32]byte, len(nodes))
					for i := range nodes {
						ids[i] = nodes[i].P2pId
					}
					return ids
				}(),
				CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
					{CapabilityId: vaultcommon.CapabilityID, Config: cfgBytes},
				},
			},
		},
		nodes: nodes,
	}

	h.vaultDONResolver = vaultdon.NewResolver(reader, "zone-a")
	h.skipVaultValidation = false
}

func encodeSignedRPCBody(t *testing.T, requestID string, payload []byte, ctxHex string, sigs ...string) []byte {
	t.Helper()

	ctxBytes, err := hex.DecodeString(ctxHex)
	require.NoError(t, err)

	signatures := make([][]byte, 0, len(sigs))
	for _, sigHex := range sigs {
		sig, err := hex.DecodeString(sigHex)
		require.NoError(t, err)
		signatures = append(signatures, sig)
	}

	result := map[string]any{
		"payload":    json.RawMessage(payload),
		"context":    ctxBytes,
		"signatures": signatures,
	}
	resultJSON, err := json.Marshal(result)
	require.NoError(t, err)

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"result":  json.RawMessage(resultJSON),
	}
	body, err := json.Marshal(resp)
	require.NoError(t, err)
	return body
}

func TestVerifyVaultGatewayResponse_SkipWhenOptedOut(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	h := &Handler{Log: &logger, skipVaultValidation: true}

	body := encodeSignedRPCBody(t, "req-1", []byte(`{"responses":[]}`),
		"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"d1067844e2849b404d903730c4cae19f090d53a578a1e8dc16ecbdc0285c1f186599108abbe0073b78bc148a6504907474ed3a6881df917e6d142cff70acfb5900",
	)

	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, "other-id", body)
	require.NoError(t, err)
}

func TestVerifyVaultGatewayResponse_RequestIDMismatch(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	h := &Handler{Log: &logger}

	body := encodeRPCBodyFromPayload(buildCreatePayloadProto(t))
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, "expected-id", body)
	require.ErrorContains(t, err, "jsonrpc id mismatch")
}

func TestVerifyVaultGatewayResponse_EmptySignaturesPassThrough(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	h := &Handler{Log: &logger}
	attachMockVaultDONResolver(t, h, "deadbeef")

	requestID := "req-empty-sigs"
	body := encodeRPCBodyFromPayload(buildCreatePayloadProto(t))
	var resp testRPCResp
	require.NoError(t, json.Unmarshal(body, &resp))
	resp.ID = requestID
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	err = h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, requestID, body)
	require.NoError(t, err)
}

func TestVerifyVaultGatewayResponse_ValidSignatures(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	h := &Handler{Log: &logger}
	attachMockVaultDONResolverWithOCRSigners(t, h, []common.Address{
		common.HexToAddress("0xd6da96fe596705b32bc3a0e11cdefad77feaad79"),
		common.HexToAddress("0x327aa349c9718cd36c877d1e90458fe1929768ad"),
		common.HexToAddress("0xe9bf394856d73402b30e160d0e05c847796f0e29"),
		common.HexToAddress("0xefd5bdb6c3256f04489a6ca32654d547297f48b9"),
	})

	requestID := "req-valid-sigs"
	payload := []byte(`{"responses":[{"error":"failed to verify ciphertext: cannot unmarshal data: unexpected end of JSON input","id":{"key":"W","namespace":"","owner":"foo"},"success":false}]}`)
	body := encodeSignedRPCBody(t, requestID, payload,
		"000ec4f6a2ba011e909eccf64628855b848e08876a1edd938a1372a9e51adff100000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000000",
		"d1067844e2849b404d903730c4cae19f090d53a578a1e8dc16ecbdc0285c1f186599108abbe0073b78bc148a6504907474ed3a6881df917e6d142cff70acfb5900",
		"c7517c188d297093a6f602046fad7feafe19454ee9dc269b19c8e6c01268037d1f7b423eeecbc495dd2d9a65e106bc3eab849ddfd74a10cbd4ad50c7d953bd4b01",
	)

	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, requestID, body)
	require.NoError(t, err)
}

func TestVerifyVaultGatewayResponse_InvalidSignatures(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	h := &Handler{Log: &logger}
	attachMockVaultDONResolverWithOCRSigners(t, h, []common.Address{
		common.HexToAddress("0xd6da96fe596705b32bc3a0e11cdefad77feaad79"),
	})

	requestID := "req-invalid-sigs"
	payload := []byte(`{"responses":[{"error":"failed to verify ciphertext: cannot unmarshal data: unexpected end of JSON input","id":{"key":"W","namespace":"","owner":"foo"},"success":false}]}`)
	body := encodeSignedRPCBody(t, requestID, payload,
		"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"d1067844e2849b404d903730c4cae19f090d53a578a1e8dc16ecbdc0285c1f186599108abbe0073b78bc148a6504907474ed3a6881df917e6d142cff70acfb5900",
		"c7517c188d297093a6f602046fad7feafe19454ee9dc269b19c8e6c01268037d1f7b423eeecbc495dd2d9a65e106bc3eab849ddfd74a10cbd4ad50c7d953bd4b01",
	)

	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, requestID, body)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "signature verification failed"))
}

func TestJSONRPCRequestID(t *testing.T) {
	id, err := jsonRPCRequestID([]byte(`{"jsonrpc":"2.0","id":"abc-123","method":"vault.secrets.list"}`))
	require.NoError(t, err)
	require.Equal(t, "abc-123", id)

	_, err = jsonRPCRequestID([]byte(`{"jsonrpc":"2.0","method":"vault.secrets.list"}`))
	require.ErrorContains(t, err, "jsonrpc request id is empty")
}
