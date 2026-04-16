package deploy

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
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

		h.environmentSet.EnvName = "STAGING"
		token := makeTestJWT(t, map[string]interface{}{
			"sub":    "user1",
			"org_id": "org-test-123",
		})
		h.credentials = makeBearerCredentials(t, token)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: "42"}
		ctx.Viper.Set("preview-private-registry", true)

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.True(t, inputs.PreviewPrivateRegistry)
	})
}

func TestCheckWorkflowExists_PrivateRegistry(t *testing.T) {
	tests := []struct {
		name         string
		serverStatus int
		response     map[string]any
		wantExists   bool
		wantStatus   *uint8
		wantErr      bool
	}{
		{
			name:         "found active workflow returns active status",
			serverStatus: http.StatusOK,
			response: map[string]any{
				"data": map[string]any{
					"getOffchainWorkflowByName": map[string]any{
						"workflow": map[string]any{
							"workflowId":     "00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87",
							"owner":          "6028e8bd8759240ffe7bd80bdd5c99ca662f3363",
							"createdAt":      "2026-04-10T14:07:25Z",
							"status":         "WORKFLOW_STATUS_ACTIVE",
							"workflowName":   "jnowak-workflow-test-v5",
							"binaryUrl":      "https://storage.cre.stage.external.griddle.sh/artifacts/00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87/binary.wasm",
							"configUrl":      "https://storage.cre.stage.external.griddle.sh/artifacts/00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87/config",
							"tag":            "",
							"attributes":     "{\"app\": \"test\"}",
							"donFamily":      "zone-a",
							"organizationId": "org_meoybOR7KEkNhEFf",
						},
					},
				},
			},
			wantExists: true,
			wantStatus: uint8Ptr(0),
			wantErr:    false,
		},
		{
			name:         "found paused workflow returns paused status",
			serverStatus: http.StatusOK,
			response: map[string]any{
				"data": map[string]any{
					"getOffchainWorkflowByName": map[string]any{
						"workflow": map[string]any{
							"workflowId":     "00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87",
							"owner":          "6028e8bd8759240ffe7bd80bdd5c99ca662f3363",
							"createdAt":      "2026-04-10T14:07:25Z",
							"status":         "WORKFLOW_STATUS_PAUSED",
							"workflowName":   "jnowak-workflow-test-v5",
							"binaryUrl":      "https://storage.cre.stage.external.griddle.sh/artifacts/00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87/binary.wasm",
							"configUrl":      "https://storage.cre.stage.external.griddle.sh/artifacts/00a2b96d2f06961c3e0cf6fbba5cfa30d3b577026de094e5202d5fc3e3aabb87/config",
							"tag":            "",
							"attributes":     "{\"app\": \"test\"}",
							"donFamily":      "zone-a",
							"organizationId": "org_meoybOR7KEkNhEFf",
						},
					},
				},
			},
			wantExists: true,
			wantStatus: uint8Ptr(1),
			wantErr:    false,
		},
		{
			name:         "not found returns no error and no status",
			serverStatus: http.StatusOK,
			response: map[string]any{
				"errors": []map[string]any{
					{
						"message": "workflow not found",
						"path":    []string{"getOffchainWorkflowByName"},
						"extensions": map[string]any{
							"code": "NOT_FOUND",
						},
					},
				},
				"data": nil,
			},
			wantExists: false,
			wantStatus: nil,
			wantErr:    false,
		},
		{
			name:         "transport failure returns error",
			serverStatus: http.StatusInternalServerError,
			response: map[string]any{
				"errors": []map[string]any{
					{
						"message": "server exploded",
					},
				},
			},
			wantExists: false,
			wantStatus: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			h := newHandler(ctx, buf)
			h.credentials = makeAPIKeyCredentials(t)

			gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
				require.True(t, containsQuery(req.Query, "query GetOffchainWorkflowByName"))
				return tt.serverStatus, tt.response
			})
			defer gqlServer.Close()

			h.environmentSet.GraphQLURL = gqlServer.URL
			strategy := newPrivateRegistryDeployStrategy(h)

			exists, status, err := strategy.CheckWorkflowExists("", "jnowak-workflow-test-v5", "", "")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
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

func uint8Ptr(v uint8) *uint8 {
	return &v
}

func TestResolveWorkflowOwner(t *testing.T) {
	t.Parallel()

	t.Run("onchain target returns configured WorkflowOwner", func(t *testing.T) {
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

		owner, err := h.resolveWorkflowOwner(registryTarget{targetType: settings.RegistryTypeOnChain})
		require.NoError(t, err)
		assert.Equal(t, chainsim.TestAddress, owner)
	})

	t.Run("private target uses derived workflow owner from runtime context", func(t *testing.T) {
		t.Parallel()

		expectedBytes, err := workflowUtils.GenerateWorkflowOwnerAddress("42", "org-test-123")
		require.NoError(t, err)
		expectedOwner := "0x" + hex.EncodeToString(expectedBytes)

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
		h.runtimeContext.DerivedWorkflowOwner = expectedOwner

		owner, err := h.resolveWorkflowOwner(registryTarget{targetType: settings.RegistryTypeOffChain})
		require.NoError(t, err)
		assert.Equal(t, expectedOwner, owner)
	})

	t.Run("private target adds 0x prefix when missing", func(t *testing.T) {
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
		h.runtimeContext.DerivedWorkflowOwner = "abcdef1234567890"

		owner, err := h.resolveWorkflowOwner(registryTarget{targetType: settings.RegistryTypeOffChain})
		require.NoError(t, err)
		assert.Equal(t, "0xabcdef1234567890", owner)
	})

	t.Run("private target errors when derived workflow owner is empty", func(t *testing.T) {
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
		h.runtimeContext.DerivedWorkflowOwner = ""

		_, err := h.resolveWorkflowOwner(registryTarget{targetType: settings.RegistryTypeOffChain})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "derived workflow owner is not available")
	})
}
