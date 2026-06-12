package common

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/gateway"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/vaultdon"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

type mockGatewayClient struct {
	post func([]byte) ([]byte, int, error)
}

func (m *mockGatewayClient) Post(b []byte) ([]byte, int, error) {
	return m.post(b)
}

func (m *mockGatewayClient) PostWithBearer(b []byte, _ string) ([]byte, int, error) {
	return m.post(b)
}

func requireZeroedBytes(t *testing.T, b []byte) {
	t.Helper()
	for i, v := range b {
		require.Zero(t, v, "byte at index %d should be zero", i)
	}
}

// It represents a hex-encoded tdh2easy.PublicKey blob.
const vaultPublicKeyHex = "7b2247726f7570223a2250323536222c22475f626172223a22424d704759487a2b33333432596436582f2b6d4971396d5468556c6d2f317355716b51783333343564303373472b2f2f307257494d39795a70454b44566c6c2b616f36586c513743366546452b665472356568785a4f343d222c2248223a22424257546f7638394b546b41505a7566474454504e35626f456d6453305368697975696e3847336e58517774454931536333394453314b41306a595a6576546155476775444d694431746e6e4d686575373177574b57593d222c22484172726179223a5b22424937726649364c646f7654413948676a684b5955516a4744456a5a66374f30774378466c432f2f384e394733464c796247436d6e54734236632b50324c34596a39477548555a4936386d54342b4e77786f794b6261513d222c22424736634369395574317a65433753786b4c442b6247354751505473717463324a7a544b4c726b784d496e4c36484e7658376541324b6167423243447a4b6a6f76783570414c6a74523734537a6c7146543366746662513d222c224245576f7147546d6b47314c31565a53655874345147446a684d4d2b656e7a6b426b7842782b484f72386e39336b51543963594938486f513630356a65504a732f53575866355a714534564e676b4f672f643530395a6b3d222c22424a31552b6e5344783269567a654177475948624e715242564869626b74466b624f4762376158562f3946744c6876314b4250416c3272696e73714171754459504e2f54667870725a6e655259594a2b2f453162536a673d222c224243675a623770424d777732337138577767736e322b6c4d665259343561347576445345715a7559614e2f356e64744970355a492f4a6f454d372b36304a6338735978682b535365364645683052364f57666855706d453d222c2242465a5942524a336d6647695644312b4f4b4e4f374c54355a6f6574515442624a6b464152757143743268492f52757832756b7166794c6c364d71566e55613557336e49726e71506132566d5345755758546d39456f733d222c22424f716b662f356232636c4d314a78615831446d6a76494c4437334f6734566b42732f4b686b6e4d6867435772552f30574a36734e514a6b425462686b4a5535576b48506342626d45786c6362706a49743349494632303d225d7d"

func TestZeroUpsertSecretValues(t *testing.T) {
	inputs := UpsertSecretsInputs{
		{ID: "a", Value: []byte("secret-one"), Namespace: "main"},
		{ID: "b", Value: []byte("secret-two"), Namespace: "main"},
	}

	ZeroUpsertSecretValues(inputs)

	for _, item := range inputs {
		requireZeroedBytes(t, item.Value)
	}
}

func TestEncryptSecrets(t *testing.T) {
	h, _, _ := newMockHandler(t)
	h.OwnerAddress = "0xabc"
	attachMockVaultDONResolver(t, h, vaultPublicKeyHex)

	t.Run("success - encrypts secrets with CapabilitiesRegistry public key", func(t *testing.T) {
		raw := UpsertSecretsInputs{
			{ID: "test-secret-1", Value: []byte("value1"), Namespace: "ns1"},
			{ID: "test-secret-2", Value: []byte("another-value"), Namespace: "ns2"},
		}

		enc, err := h.EncryptSecrets(raw, "0xabc")
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

		for i := range raw {
			requireZeroedBytes(t, raw[i].Value)
		}
	})

	t.Run("failure - CapabilitiesRegistry resolver unavailable", func(t *testing.T) {
		h2, _, _ := newMockHandler(t)
		h2.OwnerAddress = "0xabc"
		h2.TenantContext = &tenantctx.EnvironmentContext{
			CapabilitiesRegistry: &tenantctx.OnChainContract{
				ChainSelector: 16015286601757825753,
				Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
			},
		}
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")
		h2.Viper = v

		enc, err := h2.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: []byte("v"), Namespace: "n"}}, "0xabc")
		require.Error(t, err)
		require.Nil(t, enc)
		require.Contains(t, err.Error(), "encrypting secrets requires an RPC")
	})

	t.Run("failure - capabilities registry missing from user context", func(t *testing.T) {
		h2, _, _ := newMockHandler(t)
		h2.OwnerAddress = "0xabc"

		enc, err := h2.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: []byte("v"), Namespace: "n"}}, "0xabc")
		require.Error(t, err)
		require.Nil(t, enc)
		require.Contains(t, err.Error(), "capabilities registry is not configured")
	})

	t.Run("failure - vault DON resolution error", func(t *testing.T) {
		h3, _, _ := newMockHandler(t)
		h3.OwnerAddress = "0xabc"
		h3.vaultDONResolver = vaultdon.NewResolver(&mockCapRegReader{
			donIDs: []*big.Int{big.NewInt(1)},
			dons:   map[uint32]capreg.CapabilitiesRegistryDONInfo{},
		}, "zone-a")
		h3.execCtx = context.Background()

		enc, err := h3.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: []byte("v"), Namespace: "n"}}, "0xabc")
		require.Error(t, err)
		require.Nil(t, enc)
		require.Contains(t, err.Error(), "resolve vault DON")
	})
}

func TestResolveEffectiveOwner(t *testing.T) {
	t.Run("returns canonicalized workflow owner address", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.OwnerAddress = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

		owner, err := h.ResolveEffectiveOwner()
		require.NoError(t, err)
		require.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", owner)
	})

	t.Run("errors when owner address is empty", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.OwnerAddress = ""

		_, err := h.ResolveEffectiveOwner()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a valid hex address")
	})

	t.Run("errors when owner address is malformed", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.OwnerAddress = "not-an-address"

		_, err := h.ResolveEffectiveOwner()
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a valid hex address")
	})
}

func TestResolveVaultIdentifierOwnerForAuth(t *testing.T) {
	t.Run("browser returns derived workflow owner from session", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.Credentials.AuthType = credentials.AuthTypeBearer
		h.Credentials.OrgID = "org-browser"
		h.DerivedWorkflowOwner = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

		owner, err := h.ResolveVaultIdentifierOwnerForAuth(SecretsAuthBrowser)
		require.NoError(t, err)
		require.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", owner)
	})

	t.Run("browser errors on api key auth", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.Credentials.AuthType = credentials.AuthTypeApiKey
		h.Credentials.OrgID = "org-1"

		_, err := h.ResolveVaultIdentifierOwnerForAuth(SecretsAuthBrowser)
		require.Error(t, err)
		require.Contains(t, err.Error(), "interactive login")
	})

	t.Run("browser errors when derived workflow owner is empty", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.Credentials.AuthType = credentials.AuthTypeBearer
		h.Credentials.OrgID = "org-1"

		_, err := h.ResolveVaultIdentifierOwnerForAuth(SecretsAuthBrowser)
		require.Error(t, err)
		require.Contains(t, err.Error(), "derived workflow owner is not available")
	})

	t.Run("onchain delegates to ResolveEffectiveOwner", func(t *testing.T) {
		h, _, _ := newMockHandler(t)
		h.OwnerAddress = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

		owner, err := h.ResolveVaultIdentifierOwnerForAuth(SecretsAuthOnchain)
		require.NoError(t, err)
		require.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", owner)
	})
}

func TestEncryptSecrets_UsesWorkflowOwnerAddress(t *testing.T) {
	h, _, _ := newMockHandler(t)
	h.OwnerAddress = "0xabc"
	attachMockVaultDONResolver(t, h, vaultPublicKeyHex)

	enc, err := h.EncryptSecrets(UpsertSecretsInputs{
		{ID: "secret-1", Value: []byte("val1"), Namespace: "main"},
	}, "0xabc")
	require.NoError(t, err)
	require.Len(t, enc, 1)
	require.Equal(t, "0xabc", enc[0].Id.Owner)
	require.Equal(t, "secret-1", enc[0].Id.Key)
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

func TestNewHandler_WorkflowRegistryClient(t *testing.T) {
	newCtx := func(t *testing.T) (*runtime.Context, *MockClientFactory) {
		t.Helper()
		logger := zerolog.New(bytes.NewBufferString(""))
		cf := new(MockClientFactory)
		return &runtime.Context{
			Logger:        &logger,
			ClientFactory: cf,
			Settings: &settings.Settings{
				User:     settings.UserSettings{EthPrivateKey: ""},
				Workflow: settings.WorkflowSettings{},
			},
			EnvironmentSet: &environments.EnvironmentSet{GatewayURL: "http://localhost"},
			Credentials:    &credentials.Credentials{},
		}, cf
	}

	t.Run("browser flow: WorkflowRegistryV2Client is not created", func(t *testing.T) {
		ctx, cf := newCtx(t)
		h, err := NewHandler(context.Background(), ctx, "", SecretsAuthBrowser)
		require.NoError(t, err)
		require.Nil(t, h.Wrc, "Wrc must be nil for browser flow")
		cf.AssertNotCalled(t, "NewWorkflowRegistryV2Client")
	})

	t.Run("owner-key flow: WorkflowRegistryV2Client is created", func(t *testing.T) {
		ctx, cf := newCtx(t)
		cf.On("NewWorkflowRegistryV2Client", mock.Anything).Return(nil, nil)
		h, err := NewHandler(context.Background(), ctx, "", SecretsAuthOnchain)
		require.NoError(t, err)
		// Wrc may be nil if the mock returns nil, but the factory must have been called.
		_ = h
		cf.AssertCalled(t, "NewWorkflowRegistryV2Client", mock.Anything)
	})

	t.Run("owner-key flow: factory error is propagated", func(t *testing.T) {
		ctx, cf := newCtx(t)
		cf.On("NewWorkflowRegistryV2Client", mock.Anything).Return(nil, errors.New("rpc url not found for chain ethereum-mainnet"))
		_, err := NewHandler(context.Background(), ctx, "", SecretsAuthOnchain)
		require.Error(t, err)
		require.Contains(t, err.Error(), "workflow registry client")
	})
}

func TestNewHandler_GatewayURL(t *testing.T) {
	logger := zerolog.New(bytes.NewBufferString(""))
	cf := new(MockClientFactory)
	baseCtx := &runtime.Context{
		Logger:        &logger,
		ClientFactory: cf,
		Settings: &settings.Settings{
			User:     settings.UserSettings{EthPrivateKey: ""},
			Workflow: settings.WorkflowSettings{},
		},
		EnvironmentSet: &environments.EnvironmentSet{GatewayURL: "https://embedded.example.com/"},
		Credentials:    &credentials.Credentials{},
		TenantContext:  &tenantctx.EnvironmentContext{VaultGatewayURL: "https://context.example.com/"},
	}

	t.Run("uses context URL when env var unset", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, "")
		h, err := NewHandler(context.Background(), baseCtx, "", SecretsAuthBrowser)
		require.NoError(t, err)
		require.Equal(t, "https://context.example.com/", h.GatewayURL)
		gw, ok := h.Gw.(*gateway.HTTPClient)
		require.True(t, ok)
		require.Equal(t, "https://context.example.com/", gw.URL)
	})

	t.Run("env var wins over context URL", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, "https://env-override.example.com/")
		envCtx := *baseCtx
		envCtx.EnvironmentSet = &environments.EnvironmentSet{GatewayURL: "https://env-override.example.com/"}
		h, err := NewHandler(context.Background(), &envCtx, "", SecretsAuthBrowser)
		require.NoError(t, err)
		require.Equal(t, "https://env-override.example.com/", h.GatewayURL)
		gw, ok := h.Gw.(*gateway.HTTPClient)
		require.True(t, ok)
		require.Equal(t, "https://env-override.example.com/", gw.URL)
	})
}
