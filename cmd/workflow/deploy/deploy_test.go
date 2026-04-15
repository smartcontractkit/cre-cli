package deploy

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"
	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestWorkflowDeployCommand(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			inputs     Inputs
			wantErr    bool
			wantKey    string
			wantDetail string
		}{
			{
				name: "Empty Workflow Name",
				inputs: Inputs{
					WorkflowName:  "",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): ",
			},
			{
				name: "Workflow Name Too Long",
				inputs: Inputs{
					WorkflowName:  "this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			{
				name: "Invalid Workflow Owner - Missing 0x Prefix",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
			},
			{
				name: "Invalid Workflow Owner - Wrong Length",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "0x12345",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 0x12345",
			},
			{
				name: "Empty Workflow Owner",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: ",
			},
			{
				name: "Missing DonFamily",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
				},
				wantErr:    true,
				wantKey:    "Inputs.DonFamily",
				wantDetail: "DonFamily is a required field",
			},
			{
				name: "Invalid Binary URL",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
					BinaryURL:     "htp://invalid-url",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.BinaryURL",
				wantDetail: "BinaryURL must be empty or a valid HTTP URL: htp://invalid-url",
			},
			{
				name: "Invalid Config URL",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
					BinaryURL:     "https://valid-url.com/binary",
					ConfigURL:     stringPtr("htp://invalid-url"),
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.ConfigURL",
				wantDetail: "ConfigURL must be empty or a valid HTTP URL: htp://invalid-url",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()

				ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
				handler := newHandler(ctx, buf)

				ctx.Settings = createTestSettings(
					chainsim.TestAddress,
					"eoa",
					"test_workflow",
					"testdata/basic_workflow/main.go",
					"",
				)
				handler.settings = ctx.Settings

				handler.inputs = tt.inputs
				err := handler.ValidateInputs()

				if tt.wantErr {
					require.Error(t, err)
					// Ensure the error is wrapped and of type ValidationErrors
					var verrs validation.ValidationErrors
					assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")

					validation.AssertValidationErrs(t, verrs, tt.wantKey, tt.wantDetail)
					assert.False(t, handler.validated)
				}
			})
		}
	})
}

func TestResolveInputs_TagTruncation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		workflowName   string
		expectedTag    string
		expectedTagLen int
		shouldTruncate bool
	}{
		{
			name:           "short name is not truncated",
			workflowName:   "my-workflow",
			expectedTag:    "my-workflow",
			expectedTagLen: 11,
			shouldTruncate: false,
		},
		{
			name:           "exactly 32 char name is not truncated",
			workflowName:   "exactly-32-characters-long-name1",
			expectedTag:    "exactly-32-characters-long-name1",
			expectedTagLen: 32,
			shouldTruncate: false,
		},
		{
			name:           "33 char name is truncated to 32",
			workflowName:   "exactly-33-characters-long-name12",
			expectedTag:    "exactly-33-characters-long-name1",
			expectedTagLen: 32,
			shouldTruncate: true,
		},
		{
			name:           "64 char name is truncated to 32",
			workflowName:   "this-is-a-maximum-length-workflow-name-with-exactly-64-character",
			expectedTag:    "this-is-a-maximum-length-workflo",
			expectedTagLen: 32,
			shouldTruncate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			handler := newHandler(ctx, buf)

			ctx.Settings = createTestSettings(
				chainsim.TestAddress,
				"eoa",
				tt.workflowName,
				"testdata/basic_workflow/main.go",
				"",
			)
			handler.settings = ctx.Settings

			inputs, err := handler.ResolveInputs(ctx.Viper)
			require.NoError(t, err)

			assert.Equal(t, tt.workflowName, inputs.WorkflowName, "WorkflowName should always be the full name")
			assert.Equal(t, tt.expectedTag, inputs.WorkflowTag, "WorkflowTag should be truncated to 32 bytes when name exceeds limit")
			assert.Equal(t, tt.expectedTagLen, len(inputs.WorkflowTag), "WorkflowTag length mismatch")

			if tt.shouldTruncate {
				assert.NotEqual(t, inputs.WorkflowName, inputs.WorkflowTag, "tag should differ from name when truncated")
				assert.True(t, len(inputs.WorkflowName) > 32, "original name should be longer than 32")
			} else {
				assert.Equal(t, inputs.WorkflowName, inputs.WorkflowTag, "tag should equal name when not truncated")
			}
		})
	}
}

func TestResolveInputs_ConfigFlags(t *testing.T) {
	t.Parallel()

	settingsConfigPath := "testdata/basic_workflow/config.yml"
	overrideConfigPath := "testdata/basic_workflow/config.yml"

	tests := []struct {
		name               string
		viperOverrides     map[string]interface{}
		expectedConfigPath string
	}{
		{
			name:               "default uses settings config path",
			viperOverrides:     nil,
			expectedConfigPath: settingsConfigPath,
		},
		{
			name:               "no-config clears config path",
			viperOverrides:     map[string]interface{}{"no-config": true},
			expectedConfigPath: "",
		},
		{
			name:               "config flag overrides settings",
			viperOverrides:     map[string]interface{}{"config": overrideConfigPath},
			expectedConfigPath: overrideConfigPath,
		},
		{
			name:               "default-config uses settings config path",
			viperOverrides:     map[string]interface{}{"default-config": true},
			expectedConfigPath: settingsConfigPath,
		},
		{
			name:               "config flag with URL value",
			viperOverrides:     map[string]interface{}{"config": "https://example.com/config.yaml"},
			expectedConfigPath: "https://example.com/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				settingsConfigPath,
			)
			h.settings = ctx.Settings

			for k, v := range tt.viperOverrides {
				ctx.Viper.Set(k, v)
			}

			inputs, err := h.ResolveInputs(ctx.Viper)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedConfigPath, inputs.ConfigPath)
		})
	}
}

func TestResolveInputs_WasmFlag(t *testing.T) {
	t.Parallel()

	t.Run("local path", func(t *testing.T) {
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

		wasmPath := "/tmp/test.wasm"
		ctx.Viper.Set("wasm", wasmPath)

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.Equal(t, wasmPath, inputs.WasmPath)
	})

	t.Run("URL", func(t *testing.T) {
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

		wasmURL := "https://example.com/binary.wasm"
		ctx.Viper.Set("wasm", wasmURL)

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		assert.Equal(t, wasmURL, inputs.WasmPath)
	})
}

func TestValidateInputs_URLBypass(t *testing.T) {
	t.Parallel()

	t.Run("URL wasm bypasses file validation", func(t *testing.T) {
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		handler := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		handler.settings = ctx.Settings
		handler.inputs = Inputs{
			WorkflowName:                      "test_workflow",
			WorkflowOwner:                     chainsim.TestAddress,
			DonFamily:                         "test_label",
			WorkflowPath:                      "testdata/basic_workflow/main.go",
			WasmPath:                          "https://example.com/binary.wasm",
			WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
			WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
		}

		err := handler.ValidateInputs()
		require.NoError(t, err, "URL wasm path should bypass file validator")
		assert.True(t, handler.validated)
	})

	t.Run("URL config bypasses file validation", func(t *testing.T) {
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		handler := newHandler(ctx, buf)
		ctx.Settings = createTestSettings(
			chainsim.TestAddress,
			"eoa",
			"test_workflow",
			"testdata/basic_workflow/main.go",
			"",
		)
		handler.settings = ctx.Settings
		handler.inputs = Inputs{
			WorkflowName:                      "test_workflow",
			WorkflowOwner:                     chainsim.TestAddress,
			DonFamily:                         "test_label",
			WorkflowPath:                      "testdata/basic_workflow/main.go",
			ConfigPath:                        "https://example.com/config.yaml",
			WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
			WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
		}

		err := handler.ValidateInputs()
		require.NoError(t, err, "URL config path should bypass file validator")
		assert.True(t, handler.validated)
	})
}

func TestConfigFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	defer simulatedEnvironment.Close()

	cmd := New(simulatedEnvironment.NewRuntimeContext())
	cmd.SetArgs([]string{"./testdata/basic_workflow", "--no-config", "--config", "foo.yml"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group [config no-config default-config] are set none of the others can be")
}

func TestResolveInputs_PrivateRegistryTarget(t *testing.T) {
	t.Run("resolves private target and derived owner in STAGING", func(t *testing.T) {
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
		h.environmentSet.DonFamily = "test_label"
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
		assert.Equal(t, registryTargetPrivate, inputs.TargetWorkflowRegistry.targetType)
		expectedBytes, err := workflowUtils.GenerateWorkflowOwnerAddress("42", "org-test-123")
		require.NoError(t, err)
		assert.Equal(t, "0x"+hex.EncodeToString(expectedBytes), inputs.WorkflowOwner)
	})

	t.Run("rejects private target outside STAGING", func(t *testing.T) {
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
		h.environmentSet.EnvName = "PRODUCTION"
		ctx.Viper.Set("preview-private-registry", true)

		_, err := h.ResolveInputs(ctx.Viper)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--preview-private-registry is only available in the STAGING environment")
	})
}

func TestValidateInputs_PrivateRegistry(t *testing.T) {
	t.Run("accepts URL wasm and config paths for private target", func(t *testing.T) {
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
		h.environmentSet.DonFamily = "test_label"
		token := makeTestJWT(t, map[string]interface{}{
			"sub":    "user1",
			"org_id": "org-test-123",
		})
		h.credentials = makeBearerCredentials(t, token)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: "42"}
		ctx.Viper.Set("preview-private-registry", true)
		ctx.Viper.Set("wasm", "https://example.com/workflow.wasm")
		ctx.Viper.Set("config", "https://example.com/workflow-config.json")

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		h.inputs = inputs

		err = h.ValidateInputs()
		require.NoError(t, err)
		assert.True(t, h.validated)
	})

	t.Run("fails when required don family is missing for private target", func(t *testing.T) {
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
		h.environmentSet.DonFamily = ""
		token := makeTestJWT(t, map[string]interface{}{
			"sub":    "user1",
			"org_id": "org-test-123",
		})
		h.credentials = makeBearerCredentials(t, token)
		h.runtimeContext.TenantContext = &tenantctx.EnvironmentContext{TenantID: "42"}
		ctx.Viper.Set("preview-private-registry", true)
		ctx.Viper.Set("wasm", "https://example.com/workflow.wasm")

		inputs, err := h.ResolveInputs(ctx.Viper)
		require.NoError(t, err)
		h.inputs = inputs

		err = h.ValidateInputs()
		require.Error(t, err)
		var verrs validation.ValidationErrors
		assert.True(t, errors.As(err, &verrs))
		validation.AssertValidationErrs(t, verrs, "Inputs.DonFamily", "DonFamily is a required field")
	})
}

func TestExecute_PrivateRegistry(t *testing.T) {
	t.Run("executes private deploy path with GraphQL success", func(t *testing.T) {
		wasmContent := []byte("workflow wasm payload")
		wasmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(wasmContent)
		}))
		defer wasmServer.Close()

		gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
			switch {
			case req.Query != "" && containsQuery(req.Query, "query GetOffchainWorkflowByName"):
				rawRequest, ok := req.Variables["request"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test_workflow", rawRequest["workflowName"])
				return http.StatusOK, map[string]any{
					"errors": []map[string]string{{"message": "workflow not found"}},
				}
			case req.Query != "" && containsQuery(req.Query, "mutation UpsertOffchainWorkflow"):
				rawRequest, ok := req.Variables["request"].(map[string]any)
				require.True(t, ok)
				rawWorkflow, ok := rawRequest["workflow"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test_workflow", rawWorkflow["workflowName"])
				assert.Equal(t, "test-don", rawWorkflow["donFamily"])
				assert.Equal(t, wasmServer.URL+"/binary.wasm", rawWorkflow["binaryUrl"])
				assert.Equal(t, "WORKFLOW_STATUS_ACTIVE", rawWorkflow["status"])

				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"upsertOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":   "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":        chainsim.TestAddress,
								"createdAt":    "2025-01-01T00:00:00Z",
								"status":       "WORKFLOW_STATUS_ACTIVE",
								"workflowName": "test_workflow",
								"binaryUrl":    wasmServer.URL + "/binary.wasm",
								"configUrl":    "",
								"tag":          "test_workflow",
								"attributes":   "",
								"donFamily":    "test-don",
							},
						},
					},
				}
			default:
				t.Fatalf("unexpected GraphQL operation: %s", req.Query)
				return 0, nil
			}
		})
		defer gqlServer.Close()

		h := newPrivateRegistryExecuteHandler(t, wasmServer.URL+"/binary.wasm", gqlServer.URL)
		require.NoError(t, h.ValidateInputs())
		require.NoError(t, h.Execute(context.Background()))
		assert.NotEmpty(t, h.workflowArtifact.WorkflowID)
	})

	t.Run("continues when private workflow lookup returns not found", func(t *testing.T) {
		wasmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("workflow wasm payload"))
		}))
		defer wasmServer.Close()

		gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
			if containsQuery(req.Query, "query GetOffchainWorkflowByName") {
				return http.StatusOK, map[string]any{
					"errors": []map[string]string{{"message": "workflow not found"}},
				}
			}
			if containsQuery(req.Query, "mutation UpsertOffchainWorkflow") {
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"upsertOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":   "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":        chainsim.TestAddress,
								"createdAt":    "2025-01-01T00:00:00Z",
								"status":       "WORKFLOW_STATUS_ACTIVE",
								"workflowName": "test_workflow",
								"binaryUrl":    wasmServer.URL + "/binary.wasm",
								"configUrl":    "",
								"tag":          "test_workflow",
								"attributes":   "",
								"donFamily":    "test-don",
							},
						},
					},
				}
			}
			t.Fatalf("unexpected GraphQL operation: %s", req.Query)
			return 0, nil
		})
		defer gqlServer.Close()

		h := newPrivateRegistryExecuteHandler(t, wasmServer.URL+"/binary.wasm", gqlServer.URL)
		require.NoError(t, h.ValidateInputs())
		require.NoError(t, h.Execute(context.Background()))
	})

	t.Run("prompts overwrite path can proceed with skip confirmation", func(t *testing.T) {
		wasmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("workflow wasm payload"))
		}))
		defer wasmServer.Close()

		gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
			if containsQuery(req.Query, "query GetOffchainWorkflowByName") {
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"getOffchainWorkflowByName": map[string]any{
							"workflow": map[string]any{
								"workflowId":   "existing-wf-id",
								"owner":        chainsim.TestAddress,
								"createdAt":    "2025-01-01T00:00:00Z",
								"status":       "WORKFLOW_STATUS_ACTIVE",
								"workflowName": "test_workflow",
								"binaryUrl":    "https://example.com/old.wasm",
								"configUrl":    "",
								"tag":          "test_workflow",
								"attributes":   "",
								"donFamily":    "test-don",
							},
						},
					},
				}
			}
			if containsQuery(req.Query, "mutation UpsertOffchainWorkflow") {
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"upsertOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":   "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":        chainsim.TestAddress,
								"createdAt":    "2025-01-01T00:00:00Z",
								"status":       "WORKFLOW_STATUS_ACTIVE",
								"workflowName": "test_workflow",
								"binaryUrl":    wasmServer.URL + "/binary.wasm",
								"configUrl":    "",
								"tag":          "test_workflow",
								"attributes":   "",
								"donFamily":    "test-don",
							},
						},
					},
				}
			}
			t.Fatalf("unexpected GraphQL operation: %s", req.Query)
			return 0, nil
		})
		defer gqlServer.Close()

		h := newPrivateRegistryExecuteHandler(t, wasmServer.URL+"/binary.wasm", gqlServer.URL)
		h.inputs.SkipConfirmation = true
		require.NoError(t, h.ValidateInputs())
		require.NoError(t, h.Execute(context.Background()))
	})

	t.Run("surfaces GraphQL errors from private execute upsert path", func(t *testing.T) {
		wasmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("workflow wasm payload"))
		}))
		defer wasmServer.Close()

		gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
			if containsQuery(req.Query, "query GetOffchainWorkflowByName") {
				return http.StatusOK, map[string]any{
					"errors": []map[string]string{{"message": "workflow not found"}},
				}
			}
			if containsQuery(req.Query, "mutation UpsertOffchainWorkflow") {
				return http.StatusOK, map[string]any{
					"errors": []map[string]string{{"message": "unauthorized"}},
				}
			}
			t.Fatalf("unexpected GraphQL operation: %s", req.Query)
			return 0, nil
		})
		defer gqlServer.Close()

		h := newPrivateRegistryExecuteHandler(t, wasmServer.URL+"/binary.wasm", gqlServer.URL)
		require.NoError(t, h.ValidateInputs())
		err := h.Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to register workflow in private registry")
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("surfaces transport errors from private existence check", func(t *testing.T) {
		wasmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("workflow wasm payload"))
		}))
		defer wasmServer.Close()

		gqlServer := newAssertGQLServer(t, func(t *testing.T, req deployMockGraphQLRequest) (int, map[string]any) {
			if containsQuery(req.Query, "query GetOffchainWorkflowByName") {
				return http.StatusInternalServerError, map[string]any{
					"errors": []map[string]string{{"message": "server exploded"}},
				}
			}
			t.Fatalf("unexpected GraphQL operation: %s", req.Query)
			return 0, nil
		})
		defer gqlServer.Close()

		h := newPrivateRegistryExecuteHandler(t, wasmServer.URL+"/binary.wasm", gqlServer.URL)
		require.NoError(t, h.ValidateInputs())
		err := h.Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check if workflow exists")
	})
}

type deployMockGraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func newMockGQLServer(t *testing.T, response map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func newAssertGQLServer(
	t *testing.T,
	handler func(t *testing.T, req deployMockGraphQLRequest) (status int, response map[string]any),
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req deployMockGraphQLRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		status, response := handler(t, req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != nil {
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
}

func containsQuery(query, operation string) bool {
	return query != "" && strings.Contains(query, operation)
}

func newPrivateRegistryExecuteHandler(t *testing.T, wasmURL, gqlURL string) *handler {
	t.Helper()
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	t.Cleanup(simulatedEnvironment.Close)

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
	h.credentials = makeAPIKeyCredentials(t)
	h.environmentSet.GraphQLURL = gqlURL
	h.environmentSet.DonFamily = "test-don"
	h.inputs = Inputs{
		WorkflowName:                      "test_workflow",
		WorkflowOwner:                     chainsim.TestAddress,
		WorkflowTag:                       "test_workflow",
		DonFamily:                         "test-don",
		WorkflowPath:                      "testdata/basic_workflow/main.go",
		WasmPath:                          wasmURL,
		WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
		WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
		TargetWorkflowRegistry:            registryTarget{targetType: registryTargetPrivate},
		PreviewPrivateRegistry:            true,
	}

	return h
}

func stringPtr(s string) *string {
	return &s
}

type fakeUserDonLimitClient struct {
	maxAllowed           uint32
	workflowsByOwner     []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView
	workflowsByOwnerName []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView
}

func (f fakeUserDonLimitClient) CheckUserDonLimit(owner common.Address, donFamily string, pending uint32) error {
	var currentActive uint32
	for _, workflow := range f.workflowsByOwner {
		if workflow.Owner == owner && workflow.Status == workflowStatusActive && workflow.DonFamily == donFamily {
			currentActive++
		}
	}

	if currentActive+pending > f.maxAllowed {
		return errors.New("workflow limit reached")
	}
	return nil
}

func (f fakeUserDonLimitClient) GetWorkflowListByOwnerAndName(common.Address, string, *big.Int, *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	return f.workflowsByOwnerName, nil
}

func TestCheckUserDonLimitBeforeDeploy(t *testing.T) {
	owner := common.HexToAddress(chainsim.TestAddress)
	donFamily := "test-don"
	workflowName := "test-workflow"

	t.Run("errors when limit reached", func(t *testing.T) {
		client := fakeUserDonLimitClient{
			maxAllowed: 2,
			workflowsByOwner: []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
			},
		}
		nameLookup := fakeUserDonLimitClient{}

		err := checkUserDonLimitBeforeDeploy(client, nameLookup, owner, donFamily, workflowName, true, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow limit reached")
	})

	t.Run("accounts for keepAlive false pausing same-name workflows", func(t *testing.T) {
		client := fakeUserDonLimitClient{
			maxAllowed: 2,
			workflowsByOwner: []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
			},
		}
		nameLookup := fakeUserDonLimitClient{
			workflowsByOwnerName: []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
			},
		}

		err := checkUserDonLimitBeforeDeploy(client, nameLookup, owner, donFamily, workflowName, false, nil)
		require.NoError(t, err)
	})

	t.Run("skips check when updating existing workflow", func(t *testing.T) {
		client := fakeUserDonLimitClient{
			maxAllowed: 1,
			workflowsByOwner: []workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{
				{Owner: owner, Status: workflowStatusActive, DonFamily: donFamily},
			},
		}
		nameLookup := fakeUserDonLimitClient{}
		existingStatus := uint8(0)

		err := checkUserDonLimitBeforeDeploy(client, nameLookup, owner, donFamily, workflowName, true, &existingStatus)
		require.NoError(t, err)
	})
}
