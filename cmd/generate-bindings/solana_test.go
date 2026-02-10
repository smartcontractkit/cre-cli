package generatebindings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSolanaInputs_DefaultFallbacks(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create required contracts directory and go.mod
	contractsDir := filepath.Join(tempDir, "contracts")
	err = os.MkdirAll(contractsDir, 0755)
	require.NoError(t, err)

	goModPath := filepath.Join(contractsDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test/contracts\n\ngo 1.20\n"), 0600)
	require.NoError(t, err)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Test with minimal input
	v := viper.New()
	v.Set("language", "go") // Default from StringP

	inputs, err := resolveSolanaInputs(v)
	require.NoError(t, err)

	// Use filepath.EvalSymlinks to handle macOS /var vs /private/var symlink issues
	expectedRoot, _ := filepath.EvalSymlinks(tempDir)
	actualRoot, _ := filepath.EvalSymlinks(inputs.ProjectRoot)
	assert.Equal(t, expectedRoot, actualRoot)
	assert.Equal(t, "go", inputs.Language)
	expectedIdl, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "solana", "src", "idl"))
	actualIdl, _ := filepath.EvalSymlinks(inputs.IdlPath)
	assert.Equal(t, expectedIdl, actualIdl)
	expectedOut, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "solana", "src", "generated"))
	actualOut, _ := filepath.EvalSymlinks(inputs.OutPath)
	assert.Equal(t, expectedOut, actualOut)
}

// TODO: Add tests for validateSolanaInputs
// TODO: Add tests for processSolanaIdlDirectory
// TODO: Add tests for processSolanaSingleIdl
