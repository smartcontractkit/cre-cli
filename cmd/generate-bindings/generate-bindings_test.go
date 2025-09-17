package generatebindings

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func TestContractNameToPackage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"IERC20", "ierc20"},
		{"ReserveManager", "reserve_manager"},
		{"IReserveManager", "ireserve_manager"},
		{"SimpleERC20", "simple_erc20"},
		{"BalanceReader", "balance_reader"},
		{"MessageEmitter", "message_emitter"},
		{"ERC20", "erc20"},
		{"", ""},
		{"A", "a"},
		{"ABC", "abc"},
		{"HTTPClient", "http_client"},
		{"XMLParser", "xml_parser"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := contractNameToPackage(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestResolveInputs_DefaultFallbacks(t *testing.T) {
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

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test with minimal input (only chain-family)
	v := viper.New()
	v.Set("chain-family", "evm")
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	inputs, err := handler.ResolveInputs([]string{}, v)
	require.NoError(t, err)

	// Use filepath.EvalSymlinks to handle macOS /var vs /private/var symlink issues
	expectedRoot, _ := filepath.EvalSymlinks(tempDir)
	actualRoot, _ := filepath.EvalSymlinks(inputs.ProjectRoot)
	assert.Equal(t, expectedRoot, actualRoot)
	assert.Equal(t, "evm", inputs.ChainFamily)
	assert.Equal(t, "go", inputs.Language)
	expectedAbi, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "abi"))
	actualAbi, _ := filepath.EvalSymlinks(inputs.AbiPath)
	assert.Equal(t, expectedAbi, actualAbi)
	assert.Equal(t, "bindings", inputs.PkgName)
	expectedOut, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "generated"))
	actualOut, _ := filepath.EvalSymlinks(inputs.OutPath)
	assert.Equal(t, expectedOut, actualOut)
}

// command should run in projectRoot which contains contracts directory
func TestResolveInputs_CustomProjectRoot(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test with custom project root
	v := viper.New()
	v.Set("project-root", tempDir)
	v.Set("chain-family", "evm")
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	_, err = handler.ResolveInputs([]string{}, v)
	require.Error(t, err)

	expectedErrMsg := fmt.Sprintf("contracts folder not found in project root: %s", tempDir)
	require.Contains(t, err.Error(), expectedErrMsg)
}

// Empty project root should default to current directory, and this should contain contracts and go.mod
func TestResolveInputs_EmptyProjectRoot(t *testing.T) {
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

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test with empty project root (should use current directory)
	v := viper.New()
	v.Set("project-root", "")
	v.Set("chain-family", "evm")
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	inputs, err := handler.ResolveInputs([]string{}, v)
	require.NoError(t, err)

	// Use filepath.EvalSymlinks to handle macOS /var vs /private/var symlink issues
	expectedRoot, _ := filepath.EvalSymlinks(tempDir)
	actualRoot, _ := filepath.EvalSymlinks(inputs.ProjectRoot)
	assert.Equal(t, expectedRoot, actualRoot)
	assert.Equal(t, "evm", inputs.ChainFamily)
	assert.Equal(t, "go", inputs.Language)
	expectedAbi, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "abi"))
	actualAbi, _ := filepath.EvalSymlinks(inputs.AbiPath)
	assert.Equal(t, expectedAbi, actualAbi)
	assert.Equal(t, "bindings", inputs.PkgName)
	expectedOut, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "generated"))
	actualOut, _ := filepath.EvalSymlinks(inputs.OutPath)
	assert.Equal(t, expectedOut, actualOut)
}

func TestValidateInputs_RequiredChainFamily(t *testing.T) {
	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test validation with missing chain family
	inputs := Inputs{
		ProjectRoot: "/tmp",
		ChainFamily: "", // Missing required field
		Language:    "go",
		AbiPath:     "/tmp/abi",
		PkgName:     "bindings",
		OutPath:     "/tmp/out",
	}

	err := handler.ValidateInputs(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain-family")
}

func TestValidateInputs_ValidInputs(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a valid ABI file
	abiContent := `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`
	abiFile := filepath.Join(tempDir, "test.abi")
	err = os.WriteFile(abiFile, []byte(abiContent), 0600)
	require.NoError(t, err)

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test validation with valid inputs (using single file)
	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     abiFile,
		PkgName:     "bindings",
		OutPath:     tempDir,
	}

	err = handler.ValidateInputs(inputs)
	require.NoError(t, err)
	assert.True(t, handler.validated)

	// Test validation with directory containing .abi files
	abiDir := filepath.Join(tempDir, "abi")
	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(abiDir, "contract.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)

	inputs.AbiPath = abiDir
	err = handler.ValidateInputs(inputs)
	require.NoError(t, err)
	assert.True(t, handler.validated)
}

func TestValidateInputs_InvalidChainFamily(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test validation with invalid chain family
	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "solana", // No longer supported
		Language:    "go",
		AbiPath:     tempDir,
		PkgName:     "bindings",
		OutPath:     tempDir,
	}

	err = handler.ValidateInputs(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain-family")
}

func TestValidateInputs_InvalidLanguage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test validation with invalid language
	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "evm",
		Language:    "typescript", // No longer supported
		AbiPath:     tempDir,
		PkgName:     "bindings",
		OutPath:     tempDir,
	}

	err = handler.ValidateInputs(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "language")
}

func TestValidateInputs_NonExistentDirectory(t *testing.T) {
	runtimeCtx := &runtime.Context{}
	handler := newHandler(runtimeCtx)

	// Test validation with non-existent directory
	inputs := Inputs{
		ProjectRoot: "/non/existent/path",
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     "/non/existent/abi",
		PkgName:     "bindings",
		OutPath:     "/non/existent/out",
	}

	err := handler.ValidateInputs(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project-root")
}

func TestProcessAbiDirectory_MultipleFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiDir := filepath.Join(tempDir, "abi")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)

	// Create mock ABI files
	abiContent := `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`
	err = os.WriteFile(filepath.Join(abiDir, "Contract1.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(abiDir, "Contract2.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)

	// Create a mock logger to prevent nil pointer dereference
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	runtimeCtx := &runtime.Context{
		Logger: &logger,
	}
	handler := newHandler(runtimeCtx)

	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	// This test will fail because it tries to call the actual bindings.GenerateBindings
	// but it tests the directory processing logic
	err = handler.processAbiDirectory(inputs)
	// We expect an error because the bindings package requires actual ABI format
	// but we can check that it created the expected directory structure
	if err == nil {
		t.Log("Unexpectedly succeeded - bindings generation worked with mock ABI")
	} else {
		assert.Contains(t, err.Error(), "Contract1")
	}

	// Verify that per-contract directories were created
	contract1Dir := filepath.Join(outDir, "contract1")
	contract2Dir := filepath.Join(outDir, "contract2")
	assert.DirExists(t, contract1Dir)
	assert.DirExists(t, contract2Dir)
}

func TestProcessAbiDirectory_CreatesPerContractDirectories(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiDir := filepath.Join(tempDir, "abi")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)

	// Create mock ABI files with different naming patterns
	abiContent := `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`
	testCases := []struct {
		filename        string
		expectedPackage string
	}{
		{"IERC20.abi", "ierc20"},
		{"ReserveManager.abi", "reserve_manager"},
		{"SimpleERC20.abi", "simple_erc20"},
	}

	for _, tc := range testCases {
		err = os.WriteFile(filepath.Join(abiDir, tc.filename), []byte(abiContent), 0600)
		require.NoError(t, err)
	}

	// Create a mock logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	runtimeCtx := &runtime.Context{
		Logger: &logger,
	}
	handler := newHandler(runtimeCtx)

	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	// Try to process - the mock ABI content might actually work
	err = handler.processAbiDirectory(inputs)
	if err != nil {
		t.Logf("Expected error occurred: %v", err)
	}

	// Verify that per-contract directories were created with correct names
	for _, tc := range testCases {
		contractDir := filepath.Join(outDir, tc.expectedPackage)
		assert.DirExists(t, contractDir, "Expected directory %s to be created", contractDir)
	}
}

func TestProcessAbiDirectory_NoAbiFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiDir := filepath.Join(tempDir, "abi")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	runtimeCtx := &runtime.Context{
		Logger: &logger,
	}
	handler := newHandler(runtimeCtx)

	inputs := Inputs{
		ProjectRoot: tempDir,
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	err = handler.processAbiDirectory(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .abi files found")
}

func TestProcessAbiDirectory_NonExistentDirectory(t *testing.T) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	runtimeCtx := &runtime.Context{
		Logger: &logger,
	}
	handler := newHandler(runtimeCtx)

	inputs := Inputs{
		ProjectRoot: "/tmp",
		ChainFamily: "evm",
		Language:    "go",
		AbiPath:     "/non/existent/abi",
		PkgName:     "bindings",
		OutPath:     "/tmp/out",
	}

	err := handler.processAbiDirectory(inputs)
	require.Error(t, err)
	// For non-existent directory, filepath.Glob returns empty slice, so we get the "no .abi files found" error
	assert.Contains(t, err.Error(), "no .abi files found")
}
