package test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/testutil/chainsim"
)

func TestTestCmd(t *testing.T) {
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	defer simulatedEnvironment.Close()

	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			inputs  Inputs
			wantErr string
		}{
			{
				name:    "invalid test directory",
				inputs:  Inputs{TestDirectory: "not_existing_test_directory"},
				wantErr: "TestDirectory must be a valid existing directory: not_existing_test_directory",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := newHandler(simulatedEnvironment.NewRuntimeContext())
				err := handler.ValidateInputs(tt.inputs)

				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
			})
		}

		t.Run("Fails with no read access to the directory", func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			err := os.Chmod(tempDir, 0333) // Write and execute only
			require.NoError(t, err)

			inputs := Inputs{
				TestDirectory: tempDir,
			}

			handler := newHandler(simulatedEnvironment.NewRuntimeContext())
			err = handler.ValidateInputs(inputs)

			require.Error(t, err)
			assert.ErrorContains(t, err, "TestDirectory must have read access to path: "+tempDir)
		})

		t.Run("Fails when directory is a file", func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			tempFile := filepath.Join(tempDir, "testfile.txt")
			err := os.WriteFile(tempFile, []byte("test content"), 0600)
			require.NoError(t, err)

			inputs := Inputs{
				TestDirectory: tempFile,
			}

			handler := newHandler(simulatedEnvironment.NewRuntimeContext())
			err = handler.ValidateInputs(inputs)

			require.Error(t, err)
			assert.ErrorContains(t, err, "TestDirectory must be a valid existing directory")
		})
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
				args:    []string{"./dir1", "./dir2"},
				wantErr: "accepts 1 arg(s), received 2",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := New(simulatedEnvironment.NewRuntimeContext())
				cmd.SetArgs(tt.args)
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)

				err := cmd.Execute()

				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
			})
		}
	})

	t.Run("Execute", func(t *testing.T) {
		t.Run("Fails to run tests with invalid Go code", func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			brokenGoFile := filepath.Join(tempDir, "broken_test.go")
			err := os.WriteFile(brokenGoFile, []byte("package main\n func TestBroken() { invalid }"), 0600)
			require.NoError(t, err)

			err = runTestCommand(t, simulatedEnvironment, Inputs{
				TestDirectory: tempDir,
			})

			require.Error(t, err)
			assert.ErrorContains(t, err, "failed to run tests")
		})
	})
}

func runTestCommand(t *testing.T, simulatedEnvironment *chainsim.SimulatedEnvironment, inputs Inputs) error {
	handler := newHandler(simulatedEnvironment.NewRuntimeContext())
	err := handler.ValidateInputs(inputs)
	require.NoError(t, err)

	return handler.Execute(inputs)
}
