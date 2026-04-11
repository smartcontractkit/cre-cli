package deploy

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestBuildPrivateRegistryInput(t *testing.T) {
	t.Parallel()

	t.Run("maps required fields and sets active status", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			WorkflowTag:  "v1-tag",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", input.WorkflowID)
		assert.Equal(t, privateregistryclient.WorkflowStatusActive, input.Status)
		assert.Equal(t, "my-workflow", input.WorkflowName)
		assert.Equal(t, "https://storage.example.com/binary.wasm", input.BinaryURL)
		assert.Equal(t, "zone-a", input.DonFamily)
		require.NotNil(t, input.Tag)
		assert.Equal(t, "v1-tag", *input.Tag)
	})

	t.Run("includes config URL when present", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		configURL := "https://storage.example.com/config.yaml"
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			ConfigURL:    &configURL,
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		require.NotNil(t, input.ConfigURL)
		assert.Equal(t, "https://storage.example.com/config.yaml", *input.ConfigURL)
	})

	t.Run("omits config URL when absent", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Nil(t, input.ConfigURL)
	})

	t.Run("omits tag when empty", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{
			WorkflowName: "my-workflow",
			BinaryURL:    "https://storage.example.com/binary.wasm",
			DonFamily:    "zone-a",
			WorkflowTag:  "",
		}
		h.workflowArtifact = &workflowArtifact{
			WorkflowID: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		}

		input := h.buildPrivateRegistryInput()

		assert.Nil(t, input.Tag)
	})
}

func TestResolveInputs_PreviewPrivateRegistryFlag(t *testing.T) {
	t.Parallel()

	t.Run("defaults to false", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		h.settings = ctx.Settings

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.False(t, inputs.PreviewPrivateRegistry)
	})

	t.Run("set to true via viper", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		h.settings = ctx.Settings

		ctx.Viper.Set("preview-private-registry", true)

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.True(t, inputs.PreviewPrivateRegistry)
	})
}

func makeTestJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + "." + sig
}

func makeTestCredentials(t *testing.T) *credentials.Credentials {
	t.Helper()
	logger := testutil.NewTestLogger()
	prev, hadPrev := os.LookupEnv(credentials.CreApiKeyVar)
	os.Setenv(credentials.CreApiKeyVar, "test-key-for-init")
	creds, err := credentials.New(logger)
	if hadPrev {
		os.Setenv(credentials.CreApiKeyVar, prev)
	} else {
		os.Unsetenv(credentials.CreApiKeyVar)
	}
	require.NoError(t, err)
	return creds
}

func makeBearerCredentials(t *testing.T, token string) *credentials.Credentials {
	t.Helper()
	creds := makeTestCredentials(t)
	creds.AuthType = credentials.AuthTypeBearer
	creds.Tokens = &credentials.CreLoginTokenSet{AccessToken: token}
	return creds
}

func makeAPIKeyCredentials(t *testing.T) *credentials.Credentials {
	t.Helper()
	creds := makeTestCredentials(t)
	creds.AuthType = credentials.AuthTypeApiKey
	creds.APIKey = "test-key"
	return creds
}

func TestResolveWorkflowOwner(t *testing.T) {
	t.Parallel()

	t.Run("onchain target returns configured WorkflowOwner", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.inputs = Inputs{WorkflowOwner: chainsim.TestAddress}
		h.target = registryTarget{targetType: registryTargetOnchain}

		owner, err := h.resolveWorkflowOwner()
		require.NoError(t, err)
		assert.Equal(t, chainsim.TestAddress, owner)
	})

	t.Run("private target derives owner from tenantID and orgID", func(t *testing.T) {
		t.Parallel()

		token := makeTestJWT(t, map[string]interface{}{
			"sub":    "user1",
			"org_id": "org-test-123",
		})

		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.credentials = makeBearerCredentials(t, token)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: "42"}
		h.target = registryTarget{targetType: registryTargetPrivate}

		owner, err := h.resolveWorkflowOwner()
		require.NoError(t, err)

		expectedBytes, err := workflowUtils.GenerateWorkflowOwnerAddress("42", "org-test-123")
		require.NoError(t, err)
		assert.Equal(t, "0x"+hex.EncodeToString(expectedBytes), owner)
	})

	t.Run("private target errors when tenant context is nil", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.runtimeContext.TenantContext = nil
		h.target = registryTarget{targetType: registryTargetPrivate}

		_, err := h.resolveWorkflowOwner()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant context is required")
	})

	t.Run("private target errors when tenant ID is empty", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: ""}
		h.target = registryTarget{targetType: registryTargetPrivate}

		_, err := h.resolveWorkflowOwner()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tenant ID is required")
	})

	t.Run("private target errors when orgID unavailable", func(t *testing.T) {
		t.Parallel()

		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		h := newHandler(ctx, buf)
		h.credentials = makeAPIKeyCredentials(t)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: "42"}
		h.target = registryTarget{targetType: registryTargetPrivate}

		_, err := h.resolveWorkflowOwner()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get organization ID")
	})
}
