package artifact_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/artifact"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestBuilder_Build(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	artifactBuilder := artifact.NewBuilder(logger)

	t.Run("success with config", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		// Create valid base64-encoded binary file
		binaryData := []byte("test binary data")
		encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
		binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
		err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
		require.NoError(t, err)

		// Create config file
		configData := []byte("test config data")
		configPath := filepath.Join(tempDir, "config.yaml")
		err = os.WriteFile(configPath, configData, 0600)
		require.NoError(t, err)

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "test_workflow",
			OutputPath:    binaryPath,
			ConfigPath:    configPath,
		}

		art, err := artifactBuilder.Build(inputs)
		require.NoError(t, err)
		assert.NotNil(t, art)
		assert.Equal(t, []byte(encodedBinary), art.BinaryData)
		assert.Equal(t, configData, art.ConfigData)
		assert.NotEmpty(t, art.WorkflowID)
	})

	t.Run("success without config", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		// Create valid base64-encoded binary file
		binaryData := []byte("test binary data")
		encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
		binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
		err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
		require.NoError(t, err)

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "test_workflow",
			OutputPath:    binaryPath,
			ConfigPath:    "", // No config
		}

		art, err := artifactBuilder.Build(inputs)
		require.NoError(t, err)
		assert.NotNil(t, art)
		assert.Equal(t, []byte(encodedBinary), art.BinaryData)
		assert.Empty(t, art.ConfigData)
		assert.NotEmpty(t, art.WorkflowID)
	})

	t.Run("error: workflow owner is empty", func(t *testing.T) {
		t.Parallel()

		inputs := artifact.Inputs{
			WorkflowOwner: "",
			WorkflowName:  "test_workflow",
			OutputPath:    "binary.wasm",
			ConfigPath:    "",
		}

		art, err := artifactBuilder.Build(inputs)
		require.Error(t, err)
		assert.Nil(t, art)
		assert.Contains(t, err.Error(), "workflow owner is required")
	})

	t.Run("error: workflow name is empty", func(t *testing.T) {
		t.Parallel()

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "",
			OutputPath:    "binary.wasm",
			ConfigPath:    "",
		}

		art, err := artifactBuilder.Build(inputs)
		require.Error(t, err)
		assert.Nil(t, art)
		assert.Contains(t, err.Error(), "workflow name is required")
	})

	t.Run("error: invalid config path", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		// Create valid base64-encoded binary file
		binaryData := []byte("test binary data")
		encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
		binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
		err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
		require.NoError(t, err)

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "test_workflow",
			OutputPath:    binaryPath,
			ConfigPath:    "/nonexistent/config.yaml",
		}

		art, err := artifactBuilder.Build(inputs)
		require.Error(t, err)
		assert.Nil(t, art)
	})

	t.Run("error: invalid binary path", func(t *testing.T) {
		t.Parallel()

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "test_workflow",
			OutputPath:    "/nonexistent/binary.wasm",
			ConfigPath:    "",
		}

		art, err := artifactBuilder.Build(inputs)
		require.Error(t, err)
		assert.Nil(t, art)
	})

	t.Run("error: invalid base64 binary data", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		// Create invalid base64 binary file
		binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
		err := os.WriteFile(binaryPath, []byte("not-valid-base64!!!"), 0600)
		require.NoError(t, err)

		inputs := artifact.Inputs{
			WorkflowOwner: chainsim.TestAddress,
			WorkflowName:  "test_workflow",
			OutputPath:    binaryPath,
			ConfigPath:    "",
		}

		art, err := artifactBuilder.Build(inputs)
		require.Error(t, err)
		assert.Nil(t, art)
		assert.Contains(t, err.Error(), "failed to decode base64 binary data")
	})
}

func TestBuilder_BuildGeneratesConsistentWorkflowID(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	builder := artifact.NewBuilder(logger)

	tempDir := t.TempDir()

	// Create valid base64-encoded binary file
	binaryData := []byte("test binary data for consistency check")
	encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
	binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
	err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
	require.NoError(t, err)

	// Create config file
	configData := []byte("test config data for consistency")
	configPath := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configPath, configData, 0600)
	require.NoError(t, err)

	inputs := artifact.Inputs{
		WorkflowOwner: chainsim.TestAddress,
		WorkflowName:  "test_workflow",
		OutputPath:    binaryPath,
		ConfigPath:    configPath,
	}

	// Build artifact twice with same inputs
	artifact1, err := builder.Build(inputs)
	require.NoError(t, err)

	artifact2, err := builder.Build(inputs)
	require.NoError(t, err)

	// Workflow IDs should be identical for same inputs
	assert.Equal(t, artifact1.WorkflowID, artifact2.WorkflowID)
	assert.NotEmpty(t, artifact1.WorkflowID)
}

func TestBuilder_BuildGeneratesDifferentWorkflowIDsForDifferentInputs(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	builder := artifact.NewBuilder(logger)

	tempDir := t.TempDir()

	// Create valid base64-encoded binary file
	binaryData := []byte("test binary data")
	encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
	binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
	err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
	require.NoError(t, err)

	// Create config file
	configData := []byte("test config data")
	configPath := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configPath, configData, 0600)
	require.NoError(t, err)

	// Build artifact with first workflow name
	inputs1 := artifact.Inputs{
		WorkflowOwner: chainsim.TestAddress,
		WorkflowName:  "workflow_one",
		OutputPath:    binaryPath,
		ConfigPath:    configPath,
	}
	artifact1, err := builder.Build(inputs1)
	require.NoError(t, err)

	// Build artifact with different workflow name
	inputs2 := artifact.Inputs{
		WorkflowOwner: chainsim.TestAddress,
		WorkflowName:  "workflow_two",
		OutputPath:    binaryPath,
		ConfigPath:    configPath,
	}
	artifact2, err := builder.Build(inputs2)
	require.NoError(t, err)

	// Workflow IDs should be different
	assert.NotEqual(t, artifact1.WorkflowID, artifact2.WorkflowID)
	assert.NotEmpty(t, artifact1.WorkflowID)
	assert.NotEmpty(t, artifact2.WorkflowID)
}

func TestBuilder_BuildWithDifferentOwners(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	builder := artifact.NewBuilder(logger)

	tempDir := t.TempDir()

	// Create valid base64-encoded binary file
	binaryData := []byte("test binary data")
	encodedBinary := base64.StdEncoding.EncodeToString(binaryData)
	binaryPath := filepath.Join(tempDir, "binary.wasm.br.b64")
	err := os.WriteFile(binaryPath, []byte(encodedBinary), 0600)
	require.NoError(t, err)

	tests := []struct {
		name          string
		workflowOwner string
	}{
		{
			name:          "owner with 0x prefix",
			workflowOwner: chainsim.TestAddress,
		},
		{
			name:          "different owner address",
			workflowOwner: "0x37250db56cb0dd17f7653de405c89d2ac1874a63",
		},
	}

	var workflowIDs []string
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := artifact.Inputs{
				WorkflowOwner: tt.workflowOwner,
				WorkflowName:  "test_workflow",
				OutputPath:    binaryPath,
				ConfigPath:    "",
			}

			art, err := builder.Build(inputs)
			require.NoError(t, err)
			assert.NotEmpty(t, art.WorkflowID)
			workflowIDs = append(workflowIDs, art.WorkflowID)
		})
	}

	// Ensure different owners produce different workflow IDs
	if len(workflowIDs) >= 2 {
		assert.NotEqual(t, workflowIDs[0], workflowIDs[1])
	}
}
