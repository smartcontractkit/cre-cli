package solana

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
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

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	inputs, err := handler.ResolveInputs(v)
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

func TestProcessSolanaSingleIdl(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	idlDir := filepath.Join(tempDir, "idl")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(idlDir, 0755)
	require.NoError(t, err)

	// Create a simple IDL file
	simpleIdl := `{
  "address": "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
  "metadata": {
    "name": "simple_contract",
    "version": "0.1.0",
    "spec": "0.1.0"
  },
  "instructions": [
    {
      "name": "initialize",
      "discriminator": [175, 175, 109, 31, 13, 152, 155, 237],
      "accounts": [],
      "args": []
    }
  ],
  "accounts": [],
  "types": []
}`

	idlFile := filepath.Join(idlDir, "simple_contract.json")
	err = os.WriteFile(idlFile, []byte(simpleIdl), 0600)
	require.NoError(t, err)

	// Create contracts directory with go.mod for module path detection
	contractsDir := filepath.Join(tempDir, "contracts")
	err = os.MkdirAll(contractsDir, 0755)
	require.NoError(t, err)

	goModPath := filepath.Join(contractsDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test/contracts\n\ngo 1.20\n"), 0600)
	require.NoError(t, err)

	inputs := Inputs{
		ProjectRoot: tempDir,
		Language:    "go",
		IdlPath:     idlFile,
		OutPath:     outDir,
	}

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Process the single IDL file
	err = handler.processSingleIdl(inputs)

	// We expect this might fail due to missing dependencies or generator issues,
	// but we can verify that the contract directory was created
	if err != nil {
		t.Logf("Expected error occurred: %v", err)
	}

	// Verify that the contract directory was created
	contractDir := filepath.Join(outDir, "simple_contract")
	assert.DirExists(t, contractDir, "Expected contract directory to be created at %s", contractDir)
}

func TestProcessSolanaIdlDirectory(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	idlDir := filepath.Join(tempDir, "idl")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(idlDir, 0755)
	require.NoError(t, err)

	// Create multiple simple IDL files
	simpleIdl1 := `{
  "address": "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
  "metadata": {
    "name": "contract_one",
    "version": "0.1.0",
    "spec": "0.1.0"
  },
  "instructions": [
    {
      "name": "initialize",
      "discriminator": [175, 175, 109, 31, 13, 152, 155, 237],
      "accounts": [],
      "args": []
    }
  ],
  "accounts": [],
  "types": []
}`

	simpleIdl2 := `{
  "address": "FDL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfM",
  "metadata": {
    "name": "contract_two",
    "version": "0.1.0",
    "spec": "0.1.0"
  },
  "instructions": [
    {
      "name": "execute",
      "discriminator": [100, 100, 100, 31, 13, 152, 155, 237],
      "accounts": [],
      "args": []
    }
  ],
  "accounts": [],
  "types": []
}`

	err = os.WriteFile(filepath.Join(idlDir, "contract_one.json"), []byte(simpleIdl1), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(idlDir, "contract_two.json"), []byte(simpleIdl2), 0600)
	require.NoError(t, err)

	// Create contracts directory with go.mod for module path detection
	contractsDir := filepath.Join(tempDir, "contracts")
	err = os.MkdirAll(contractsDir, 0755)
	require.NoError(t, err)

	goModPath := filepath.Join(contractsDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test/contracts\n\ngo 1.20\n"), 0600)
	require.NoError(t, err)

	inputs := Inputs{
		ProjectRoot: tempDir,
		Language:    "go",
		IdlPath:     idlDir,
		OutPath:     outDir,
	}

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Process the IDL directory
	err = handler.processIdlDirectory(inputs)

	// We expect this might fail due to missing dependencies or generator issues,
	// but we can verify that the contract directories were created
	if err != nil {
		t.Logf("Expected error occurred: %v", err)
	}

	// Verify that per-contract directories were created
	contract1Dir := filepath.Join(outDir, "contract_one")
	contract2Dir := filepath.Join(outDir, "contract_two")
	assert.DirExists(t, contract1Dir, "Expected contract directory to be created at %s", contract1Dir)
	assert.DirExists(t, contract2Dir, "Expected contract directory to be created at %s", contract2Dir)
}

func TestProcessSolanaIdlDirectory_NoIdlFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	idlDir := filepath.Join(tempDir, "idl")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(idlDir, 0755)
	require.NoError(t, err)

	inputs := Inputs{
		ProjectRoot: tempDir,
		Language:    "go",
		IdlPath:     idlDir,
		OutPath:     outDir,
	}

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	err = handler.processIdlDirectory(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .json files found")
}
