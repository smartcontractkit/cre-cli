package deploy

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestCompileCmd(t *testing.T) {
	t.Run("input errors", func(t *testing.T) {
		tests := []struct {
			name              string
			cmd               Inputs
			wantKeys          []string
			wantDetails       []string
			WorkflowOwnerType string
			wantError         bool
		}{
			{
				name:        "Required WorkflowPath",
				cmd:         Inputs{},
				wantError:   true,
				wantKeys:    []string{"Inputs.WorkflowPath"},
				wantDetails: []string{"WorkflowPath is a required field"},
			},
			{
				name: "Invalid WorkflowPath",
				cmd: Inputs{
					WorkflowPath: "nonexistent.yaml",
				},
				wantError:   true,
				wantKeys:    []string{"Inputs.WorkflowPath"},
				wantDetails: []string{"WorkflowPath must have read access to path: nonexistent.yaml"},
			},
			{
				name: "Invalid ConfigPath",
				cmd: Inputs{
					WorkflowPath:                      "testdata/test_workflow.yaml",
					ConfigPath:                        "nonexistent.yaml",
					WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.ConfigPath"},
				wantDetails:       []string{"--config must be a valid existing file: nonexistent.yaml"},
			},
			{
				name: "Non-ASCII ConfigPath",
				cmd: Inputs{
					WorkflowPath:                      "testdata/test_workflow.yaml",
					ConfigPath:                        "./testdata/đuveč.yaml",
					WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.ConfigPath"},
				wantDetails:       []string{"--config must contain only ASCII characters: ./testdata/đuveč.yaml"},
			},
			{
				name: "Non-ASCII OutputPath",
				cmd: Inputs{
					WorkflowPath:                      "testdata/test_workflow.yaml",
					OutputPath:                        "outputŠČ.yaml",
					WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.OutputPath"},
				wantDetails:       []string{"--output must contain only ASCII characters: outputŠČ.yaml"},
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
					tt.WorkflowOwnerType,
					"test_workflow",
					tt.cmd.WorkflowPath,
					tt.cmd.ConfigPath,
				)
				handler.settings = ctx.Settings
				handler.inputs = tt.cmd
				err := handler.ValidateInputs()

				if tt.wantError {
					assert.Error(t, err, "Expected validation error")

					var verrs validation.ValidationErrors
					assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")

					for i := range tt.wantKeys {
						validation.AssertValidationErrs(t, verrs, tt.wantKeys[i], tt.wantDetails[i])
					}
				} else {
					assert.NoError(t, err, "Did not expect validation error")
				}
			})
		}
	})

	t.Run("args errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			args    []string
			wantErr string
		}{
			{
				name:    "no args provided",
				args:    []string{},
				wantErr: "accepts 1 arg(s), received 0",
			},
			{
				name:    "too many args",
				args:    []string{"file1.go", "file2.go"},
				wantErr: "accepts 1 arg(s), received 2",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()

				cmd := New(simulatedEnvironment.NewRuntimeContext())
				cmd.SetArgs(tt.args)
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				err := cmd.Execute()
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
			})
		}
	})

	t.Run("compile", func(t *testing.T) {
		outputFileName := "binary.wasm.br.b64"
		outputPath := "./" + outputFileName

		t.Run("errors", func(t *testing.T) {
			httpmock.Activate()
			t.Cleanup(httpmock.DeactivateAndReset)

			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			err := runCompile(simulatedEnvironment, Inputs{
				WorkflowName:                      "test_workflow",
				WorkflowOwner:                     chainsim.TestAddress,
				DonFamily:                         "test_label",
				WorkflowPath:                      filepath.Join("testdata", "malformed_workflow", "main.go"),
				OutputPath:                        outputPath,
				WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
				WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
			}, constants.WorkflowOwnerTypeEOA)
			require.Error(t, err)
			assert.ErrorContains(t, err, "failed to compile workflow")
			assert.ErrorContains(t, err, "undefined: sdk.RemovedFunctionThatFailsCompilation")
		})
	})
}

func TestCompileOutputMatchesUnderlying(t *testing.T) {
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	baseInputs := Inputs{
		WorkflowName:                      "test_workflow",
		WorkflowOwner:                     chainsim.TestAddress,
		DonFamily:                         "test_label",
		WorkflowPath:                      filepath.Join("testdata", "basic_workflow", "main.go"),
		ConfigPath:                        filepath.Join("testdata", "basic_workflow", "config.yml"),
		WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
		WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
	}

	t.Run("default output path", func(t *testing.T) {
		inputs := baseInputs
		inputs.OutputPath = "./binary.wasm.br.b64"
		assertCompileOutputMatchesUnderlying(t, simulatedEnvironment, inputs, constants.WorkflowOwnerTypeEOA)
	})

	t.Run("output path extension variants", func(t *testing.T) {
		tests := []struct {
			name       string
			outputPath string
		}{
			{"no extension", "./my-binary"},
			{"missing .br and .b64", "./my-binary.wasm"},
			{"missing .b64", "./my-binary.wasm.br"},
			{"all extensions", "./my-binary.wasm.br.b64"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				inputs := baseInputs
				inputs.OutputPath = tt.outputPath
				assertCompileOutputMatchesUnderlying(t, simulatedEnvironment, inputs, constants.WorkflowOwnerTypeEOA)
			})
		}
	})
}

// createTestSettings is a helper function to construct settings for tests
func createTestSettings(workflowOwnerAddress, workflowOwnerType, workflowName, workflowPath, configPath string) *settings.Settings {
	return &settings.Settings{
		Workflow: settings.WorkflowSettings{
			UserWorkflowSettings: struct {
				WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
				WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
				WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
			}{
				WorkflowOwnerAddress: workflowOwnerAddress,
				WorkflowOwnerType:    workflowOwnerType,
				WorkflowName:         workflowName,
			},
			WorkflowArtifactSettings: struct {
				WorkflowPath string `mapstructure:"workflow-path" yaml:"workflow-path"`
				ConfigPath   string `mapstructure:"config-path" yaml:"config-path"`
				SecretsPath  string `mapstructure:"secrets-path" yaml:"secrets-path"`
			}{
				WorkflowPath: workflowPath,
				ConfigPath:   configPath,
			},
		},
		StorageSettings: settings.WorkflowStorageSettings{
			CREStorage: settings.CREStorageSettings{
				ServiceTimeout: 0,
				HTTPTimeout:    0,
			},
		},
	}
}

func runCompile(simulatedEnvironment *chainsim.SimulatedEnvironment, inputs Inputs, ownerType string) error {
	ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
	handler := newHandler(ctx, buf)

	ctx.Settings = createTestSettings(
		inputs.WorkflowOwner,
		ownerType,
		inputs.WorkflowName,
		inputs.WorkflowPath,
		inputs.ConfigPath,
	)
	handler.settings = ctx.Settings

	handler.inputs = inputs
	err := handler.ValidateInputs()
	if err != nil {
		return err
	}

	return handler.Compile()
}

// outputPathWithExtensions returns the path with .wasm.br.b64 appended as in Compile().
func outputPathWithExtensions(path string) string {
	if path == "" {
		path = defaultOutputPath
	}
	if !strings.HasSuffix(path, ".b64") {
		if !strings.HasSuffix(path, ".br") {
			if !strings.HasSuffix(path, ".wasm") {
				path += ".wasm"
			}
			path += ".br"
		}
		path += ".b64"
	}
	return path
}

// assertCompileOutputMatchesUnderlying compiles via handler.Compile(), then verifies the output
// file content equals CompileWorkflowToWasm(workflowPath) + brotli + base64.
func assertCompileOutputMatchesUnderlying(t *testing.T, simulatedEnvironment *chainsim.SimulatedEnvironment, inputs Inputs, ownerType string) {
	t.Helper()
	wasm, err := cmdcommon.CompileWorkflowToWasm(inputs.WorkflowPath)
	require.NoError(t, err)
	compressed, err := applyBrotliCompressionV2(&wasm)
	require.NoError(t, err)
	expected := base64.StdEncoding.EncodeToString(compressed)

	err = runCompile(simulatedEnvironment, inputs, ownerType)
	require.NoError(t, err)

	actualPath := outputPathWithExtensions(inputs.OutputPath)
	t.Cleanup(func() { _ = os.Remove(actualPath) })
	actual, err := os.ReadFile(actualPath)
	require.NoError(t, err)
	assert.Equal(t, expected, string(actual), "handler.Compile() output should match CompileWorkflowToWasm + brotli + base64")
}

// TestCustomWasmWorkflowRunsMakeBuild ensures that simulate/deploy run "make build" for a custom
// WASM workflow (workflow-path pointing to .wasm) so the user does not need to run make build manually.
func TestCustomWasmWorkflowRunsMakeBuild(t *testing.T) {
	customWasmDir := filepath.Join("testdata", "custom_wasm_workflow")
	wasmPath := filepath.Join(customWasmDir, "wasm", "workflow.wasm")

	// Remove wasm file if present so we assert the CLI builds it (CompileWorkflowToWasm runs make via ensureWasmBuilt).
	_ = os.Remove(wasmPath)
	t.Cleanup(func() { _ = os.Remove(wasmPath) })

	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	defer simulatedEnvironment.Close()

	outputPath := filepath.Join(customWasmDir, "test_out.wasm.br.b64")
	t.Cleanup(func() { _ = os.Remove(outputPath) })

	inputs := Inputs{
		WorkflowName:                      "custom_wasm_workflow",
		WorkflowOwner:                     chainsim.TestAddress,
		DonFamily:                         "test_label",
		WorkflowPath:                      wasmPath,
		ConfigPath:                        filepath.Join(customWasmDir, "config.yml"),
		WorkflowRegistryContractAddress:   "0x1234567890123456789012345678901234567890",
		WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
		OutputPath:                        outputPath,
	}

	// runCompile calls ValidateInputs then Compile; CompileWorkflowToWasm runs make build internally. No manual make build.
	err := runCompile(simulatedEnvironment, inputs, constants.WorkflowOwnerTypeEOA)
	require.NoError(t, err, "custom WASM workflow should build via CLI (CompileWorkflowToWasm) without manual make build")

	// Ensure the wasm was actually built by the CLI
	_, err = os.Stat(wasmPath)
	require.NoError(t, err, "wasm/workflow.wasm should exist after compile")
}
