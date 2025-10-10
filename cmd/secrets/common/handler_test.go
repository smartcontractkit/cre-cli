package common

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"
)

type mockGatewayClient struct {
	post func([]byte) ([]byte, int, error)
}

func (m *mockGatewayClient) Post(b []byte) ([]byte, int, error) {
	return m.post(b)
}

// It represents a hex-encoded tdh2easy.PublicKey blob.
const vaultPublicKeyHex = "7b2247726f7570223a2250323536222c22475f626172223a22424d704759487a2b33333432596436582f2b6d4971396d5468556c6d2f317355716b51783333343564303373472b2f2f307257494d39795a70454b44566c6c2b616f36586c513743366546452b665472356568785a4f343d222c2248223a22424257546f7638394b546b41505a7566474454504e35626f456d6453305368697975696e3847336e58517774454931536333394453314b41306a595a6576546155476775444d694431746e6e4d686575373177574b57593d222c22484172726179223a5b22424937726649364c646f7654413948676a684b5955516a4744456a5a66374f30774378466c432f2f384e394733464c796247436d6e54734236632b50324c34596a39477548555a4936386d54342b4e77786f794b6261513d222c22424736634369395574317a65433753786b4c442b6247354751505473717463324a7a544b4c726b784d496e4c36484e7658376541324b6167423243447a4b6a6f76783570414c6a74523734537a6c7146543366746662513d222c224245576f7147546d6b47314c31565a53655874345147446a684d4d2b656e7a6b426b7842782b484f72386e39336b51543963594938486f513630356a65504a732f53575866355a714534564e676b4f672f643530395a6b3d222c22424a31552b6e5344783269567a654177475948624e715242564869626b74466b624f4762376158562f3946744c6876314b4250416c3272696e73714171754459504e2f54667870725a6e655259594a2b2f453162536a673d222c224243675a623770424d777732337138577767736e322b6c4d665259343561347576445345715a7559614e2f356e64744970355a492f4a6f454d372b36304a6338735978682b535365364645683052364f57666855706d453d222c2242465a5942524a336d6647695644312b4f4b4e4f374c54355a6f6574515442624a6b464152757143743268492f52757832756b7166794c6c364d71566e55613557336e49726e71506132566d5345755758546d39456f733d222c22424f716b662f356232636c4d314a78615831446d6a76494c4437334f6734566b42732f4b686b6e4d6867435772552f30574a36734e514a6b425462686b4a5535576b48506342626d45786c6362706a49743349494632303d225d7d"

func TestEncryptSecrets(t *testing.T) {
	h, _, _ := newMockHandler(t)
	h.OwnerAddress = "0xabc"

	t.Run("success - encrypts secrets with a gateway-fetched public key", func(t *testing.T) {
		h.Gw = &mockGatewayClient{
			post: func(body []byte) ([]byte, int, error) {
				// Echo a valid JSON-RPC response with matching ID/method
				var req jsonrpc2.Request[vaultcommon.GetPublicKeyRequest]
				require.NoError(t, json.Unmarshal(body, &req))
				require.Equal(t, jsonrpc2.JsonRpcVersion, req.Version)
				require.Equal(t, vaulttypes.MethodPublicKeyGet, req.Method)

				resp := jsonrpc2.Response[vaultcommon.GetPublicKeyResponse]{
					Version: jsonrpc2.JsonRpcVersion,
					ID:      req.ID,
					Method:  vaulttypes.MethodPublicKeyGet,
					Result:  &vaultcommon.GetPublicKeyResponse{PublicKey: vaultPublicKeyHex},
				}
				b, _ := json.Marshal(resp)
				return b, http.StatusOK, nil
			},
		}

		raw := UpsertSecretsInputs{
			{ID: "test-secret-1", Value: "value1", Namespace: "ns1"},
			{ID: "test-secret-2", Value: "another-value", Namespace: "ns2"},
		}

		enc, err := h.EncryptSecrets(raw)
		require.NoError(t, err)
		require.Len(t, enc, 2)

		require.Equal(t, "test-secret-1", enc[0].Id.Key)
		require.Equal(t, "ns1", enc[0].Id.Namespace)
		require.Equal(t, "0xabc", enc[0].Id.Owner)

		require.Equal(t, "test-secret-2", enc[1].Id.Key)
		require.Equal(t, "ns2", enc[1].Id.Namespace)
		require.Equal(t, "0xabc", enc[1].Id.Owner)

		// We can't (and don't need to) decrypt here; just assert it's valid hex and non-empty.
		_, err = hex.DecodeString(enc[0].EncryptedValue)
		require.NoError(t, err)
		require.NotEmpty(t, enc[0].EncryptedValue)

		_, err = hex.DecodeString(enc[1].EncryptedValue)
		require.NoError(t, err)
		require.NotEmpty(t, enc[1].EncryptedValue)
	})

	t.Run("failure - gateway POST error", func(t *testing.T) {
		h.Gw = &mockGatewayClient{
			post: func(_ []byte) ([]byte, int, error) {
				return nil, 0, errors.New("network down")
			},
		}

		enc, err := h.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: "v", Namespace: "n"}})
		require.Error(t, err)
		require.Nil(t, enc)
		require.Contains(t, err.Error(), "gateway POST failed")
	})

	t.Run("failure - JSON-RPC error from gateway", func(t *testing.T) {
		h.Gw = &mockGatewayClient{
			post: func(body []byte) ([]byte, int, error) {
				var req jsonrpc2.Request[vaultcommon.GetPublicKeyRequest]
				_ = json.Unmarshal(body, &req)

				resp := map[string]any{
					"jsonrpc": jsonrpc2.JsonRpcVersion,
					"id":      req.ID,
					"method":  vaulttypes.MethodPublicKeyGet,
					"error": map[string]any{
						"code":    -32000,
						"message": "pk error",
					},
				}
				b, _ := json.Marshal(resp)
				return b, http.StatusOK, nil
			},
		}

		enc, err := h.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: "v", Namespace: "n"}})
		require.Error(t, err)
		require.Nil(t, enc)
		require.Contains(t, err.Error(), "vault public key fetch error")
	})
}

func TestPackAllowlistRequestTxData_Success_With0x(t *testing.T) {
	h, _, _ := newMockHandler(t)

	// random 32-byte digest
	var d [32]byte
	_, err := rand.Read(d[:])
	require.NoError(t, err)

	dur := 15 * time.Minute
	start := time.Now().Unix()

	// call
	dataHex, err := h.PackAllowlistRequestTxData(d, dur)
	require.NoError(t, err)
	require.NotEmpty(t, dataHex)

	// decode and verify selector + args
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	abiSpec, err := abi.JSON(strings.NewReader(workflow_registry_wrapper_v2.WorkflowRegistryMetaData.ABI))
	require.NoError(t, err)

	m := abiSpec.Methods["allowlistRequest"]
	require.NotNil(t, m)

	// first 4 bytes: function selector
	require.GreaterOrEqual(t, len(data), 4)
	assert.Equal(t, m.ID[:], data[:4], "method selector must match")

	// unpack args (skip selector)
	args, err := m.Inputs.Unpack(data[4:])
	require.NoError(t, err)
	require.Len(t, args, 2)

	// arg0: bytes32
	gotDigest, ok := args[0].([32]byte)
	require.True(t, ok)
	assert.Equal(t, d, gotDigest)

	// arg1: uint32 expiry ~ now + dur (allow small skew)
	gotExpiry, ok := args[1].(uint32)
	require.Truef(t, ok, "expiry should be uint32, got %T", args[1])

	exp := int64(gotExpiry) // seconds since epoch
	target := start + int64(dur/time.Second)

	assert.GreaterOrEqual(t, exp, target-5)
	assert.LessOrEqual(t, exp, target+60)
}

func TestPackAllowlistRequestTxData_Success_No0x(t *testing.T) {
	h, _, _ := newMockHandler(t)

	var d [32]byte
	_, err := rand.Read(d[:])
	require.NoError(t, err)

	dataHex, err := h.PackAllowlistRequestTxData(d, 1*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, dataHex)
}
