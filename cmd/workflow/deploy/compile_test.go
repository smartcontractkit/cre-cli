package deploy

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/gist"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestCompileCmd(t *testing.T) {
	t.Run("input errors", func(t *testing.T) {
		gist.SetupGitHubAPIMocks(t, "valid-token", "foo")
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
				wantDetails: []string{"WorkflowPath must be a valid existing file: nonexistent.yaml"},
			},
			{
				name: "Invalid ConfigPath",
				cmd: Inputs{
					WorkflowPath:                          "testdata/test_workflow.yaml",
					ConfigPath:                            "nonexistent.yaml",
					WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainselector: 1234567890,
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.ConfigPath"},
				wantDetails:       []string{"--config must be a valid existing file: nonexistent.yaml"},
			},
			{
				name: "Non-ASCII ConfigPath",
				cmd: Inputs{
					WorkflowPath:                          "testdata/test_workflow.yaml",
					ConfigPath:                            "./testdata/đuveč.yaml",
					WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainselector: 1234567890,
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.ConfigPath"},
				wantDetails:       []string{"--config must contain only ASCII characters: ./testdata/đuveč.yaml"},
			},
			{
				name: "Non-ASCII OutputPath",
				cmd: Inputs{
					WorkflowPath:                          "testdata/test_workflow.yaml",
					OutputPath:                            "outputŠČ.yaml",
					WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainselector: 1234567890,
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         true,
				wantKeys:          []string{"Inputs.OutputPath"},
				wantDetails:       []string{"--output must contain only ASCII characters: outputŠČ.yaml"},
			},
			{
				name: "Valid Input Without Gist",
				cmd: Inputs{
					WorkflowName:                          "test_workflow",
					WorkflowOwner:                         chainsim.TestAddress,
					DonFamily:                             "test_label",
					WorkflowPath:                          filepath.Join("testdata", "basic_workflow", "main.go"),
					ConfigPath:                            filepath.Join("testdata", "basic_workflow", "config.yml"),
					OutputPath:                            "output.yaml",
					WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainselector: 1234567890,
				},
				WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
				wantError:         false,
				wantKeys:          []string{},
				wantDetails:       []string{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()

				ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
				handler := newHandler(ctx, buf)
				handler.inputs = tt.cmd
				ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = tt.WorkflowOwnerType
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
			gist.SetupGistAPIMock(t, "valid-token", "foo")

			tests := []struct {
				inputs            Inputs
				wantErr           string
				compilationErr    string
				WorkflowOwnerType string
			}{
				{
					inputs: Inputs{
						WorkflowName:                          "test_workflow",
						WorkflowOwner:                         chainsim.TestAddress,
						DonFamily:                             "test_label",
						WorkflowPath:                          filepath.Join("testdata", "malformed_workflow", "main.go"),
						OutputPath:                            outputPath,
						WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
						WorkflowRegistryContractChainselector: 1234567890,
					},
					WorkflowOwnerType: constants.WorkflowOwnerTypeEOA,
					wantErr:           "failed to compile workflow: exit status 1",
					compilationErr:    "undefined: sdk.RemovedFunctionThatFailsCompilation",
				},
			}

			for _, tt := range tests {
				t.Run(tt.wantErr, func(t *testing.T) {
					simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
					defer simulatedEnvironment.Close()

					ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
					handler := newHandler(ctx, buf)
					handler.inputs = tt.inputs
					ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = tt.WorkflowOwnerType
					err := handler.ValidateInputs()
					require.NoError(t, err)

					err = handler.Execute()
					require.Error(t, err)
					assert.ErrorContains(t, err, tt.wantErr)

					if tt.compilationErr != "" {
						assert.Contains(t, buf.String(), tt.compilationErr)
					}
				})
			}
		})

		t.Run("no config", func(t *testing.T) {
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = constants.WorkflowOwnerTypeEOA

			httpmock.Activate()
			t.Cleanup(httpmock.DeactivateAndReset)
			gist.SetupGistAPIMock(t, "valid-token", "foo")

			err := runCompile(simulatedEnvironment, Inputs{
				WorkflowName:                          "test_workflow",
				WorkflowOwner:                         chainsim.TestAddress,
				DonFamily:                             "test_label",
				WorkflowPath:                          filepath.Join("testdata", "configless_workflow", "main.go"),
				OutputPath:                            outputPath,
				WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
				WorkflowRegistryContractChainselector: 1234567890,
			}, constants.WorkflowOwnerTypeEOA)
			defer os.Remove(outputPath)

			require.NoError(t, err)
		})

		t.Run("with config", func(t *testing.T) {
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			err := runCompile(simulatedEnvironment, Inputs{
				WorkflowName:                          "test_workflow",
				WorkflowOwner:                         chainsim.TestAddress,
				DonFamily:                             "test_label",
				WorkflowPath:                          filepath.Join("testdata", "basic_workflow", "main.go"),
				OutputPath:                            outputPath,
				ConfigPath:                            filepath.Join("testdata", "basic_workflow", "config.yml"),
				WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
				WorkflowRegistryContractChainselector: 1234567890,
			}, constants.WorkflowOwnerTypeEOA)
			defer os.Remove(outputPath)

			require.NoError(t, err)
		})

		t.Run("compiles even without go.mod", func(t *testing.T) {
			// it auto falls back to using the go.mod in the root directory (/cre-cli)
			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			httpmock.Activate()
			t.Cleanup(httpmock.DeactivateAndReset)
			gist.SetupGistAPIMock(t, "valid-token", "foo")

			err := runCompile(simulatedEnvironment, Inputs{
				WorkflowName:                          "test_workflow",
				WorkflowOwner:                         chainsim.TestAddress,
				DonFamily:                             "test_label",
				WorkflowPath:                          filepath.Join("testdata", "missing_go_mod", "main.go"),
				OutputPath:                            outputPath,
				WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
				WorkflowRegistryContractChainselector: 1234567890,
			}, constants.WorkflowOwnerTypeEOA)
			defer os.Remove(outputPath)

			require.NoError(t, err)
		})

	})
}

func TestCompileCreatesBase64EncodedFile(t *testing.T) {
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)

	t.Run("default output file is binary.wasm.br", func(t *testing.T) {
		expectedOutputPath := "./binary.wasm.br.b64"

		err := runCompile(simulatedEnvironment, Inputs{
			WorkflowName:                          "test_workflow",
			WorkflowOwner:                         chainsim.TestAddress,
			DonFamily:                             "test_label",
			WorkflowPath:                          filepath.Join("testdata", "basic_workflow", "main.go"),
			ConfigPath:                            filepath.Join("testdata", "basic_workflow", "config.yml"),
			WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
			WorkflowRegistryContractChainselector: 1234567890,
		}, constants.WorkflowOwnerTypeEOA)
		defer os.Remove(expectedOutputPath)

		require.NoError(t, err)
		assert.FileExists(t, expectedOutputPath)
	})

	t.Run("ensures output file has .wasm.br.b64 extension", func(t *testing.T) {
		tests := []struct {
			name           string
			outputPath     string
			expectedOutput string
		}{
			{
				name:           "no extension",
				outputPath:     "./my-binary",
				expectedOutput: "./my-binary.wasm.br.b64",
			},
			{
				name:           "missing .br and .b64",
				outputPath:     "./my-binary.wasm",
				expectedOutput: "./my-binary.wasm.br.b64",
			},
			{
				name:           "missing .b64",
				outputPath:     "./my-binary.wasm.br",
				expectedOutput: "./my-binary.wasm.br.b64",
			},
			{
				name:           "all extensions",
				outputPath:     "./my-binary.wasm.br.b64",
				expectedOutput: "./my-binary.wasm.br.b64",
			},
			{
				name:           "all extensions - same as default",
				outputPath:     "./binary.wasm.br.b64",
				expectedOutput: "./binary.wasm.br.b64",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := runCompile(simulatedEnvironment, Inputs{
					WorkflowName:                          "test_workflow",
					WorkflowOwner:                         chainsim.TestAddress,
					DonFamily:                             "test_label",
					WorkflowPath:                          filepath.Join("testdata", "basic_workflow", "main.go"),
					ConfigPath:                            filepath.Join("testdata", "basic_workflow", "config.yml"),
					OutputPath:                            tt.outputPath,
					WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
					WorkflowRegistryContractChainselector: 1234567890,
				}, constants.WorkflowOwnerTypeEOA)
				defer os.Remove(tt.expectedOutput)

				require.NoError(t, err)
				assert.FileExists(t, tt.expectedOutput)
			})
		}
	})

	t.Run("output file is base64 encoded", func(t *testing.T) {
		outputPath := "./binary.wasm.br.b64"

		err := runCompile(simulatedEnvironment, Inputs{
			WorkflowName:                          "test_workflow",
			WorkflowOwner:                         chainsim.TestAddress,
			DonFamily:                             "test_label",
			WorkflowPath:                          filepath.Join("testdata", "basic_workflow", "main.go"),
			ConfigPath:                            filepath.Join("testdata", "basic_workflow", "config.yml"),
			OutputPath:                            outputPath,
			WorkflowRegistryContractAddress:       "0x1234567890123456789012345678901234567890",
			WorkflowRegistryContractChainselector: 1234567890,
		}, constants.WorkflowOwnerTypeEOA)
		defer os.Remove(outputPath)

		require.NoError(t, err)
		assert.FileExists(t, outputPath)

		// Read the output file content
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		// Check if the content is valid base64
		_, err = base64.StdEncoding.DecodeString(string(content))
		assert.NoError(t, err, "Output file content should be valid base64 encoded data")
	})
}

func runCompile(simulatedEnvironment *chainsim.SimulatedEnvironment, inputs Inputs, ownerType string) error {
	ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
	ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = ownerType
	handler := newHandler(ctx, buf)
	handler.inputs = inputs
	err := handler.ValidateInputs()
	if err != nil {
		return err
	}

	return handler.Compile()
}
