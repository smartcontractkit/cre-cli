package evm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResolveEvmInputs_DefaultFallbacks(t *testing.T) {
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
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	inputs, err := resolveEvmInputs(v)
	require.NoError(t, err)

	// Use filepath.EvalSymlinks to handle macOS /var vs /private/var symlink issues
	expectedRoot, _ := filepath.EvalSymlinks(tempDir)
	actualRoot, _ := filepath.EvalSymlinks(inputs.ProjectRoot)
	assert.Equal(t, expectedRoot, actualRoot)
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
func TestResolveEvmInputs_CustomProjectRoot(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test with custom project root
	v := viper.New()
	v.Set("project-root", tempDir)
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	_, err = resolveEvmInputs(v)
	require.Error(t, err)

	expectedErrMsg := fmt.Sprintf("contracts folder not found in project root: %s", tempDir)
	require.Contains(t, err.Error(), expectedErrMsg)
}

// Empty project root should default to current directory, and this should contain contracts and go.mod
func TestResolveEvmInputs_EmptyProjectRoot(t *testing.T) {
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

	// Test with empty project root (should use current directory)
	v := viper.New()
	v.Set("project-root", "")
	v.Set("language", "go")  // Default from StringP
	v.Set("pkg", "bindings") // Default from StringP

	inputs, err := resolveEvmInputs(v)
	require.NoError(t, err)

	// Use filepath.EvalSymlinks to handle macOS /var vs /private/var symlink issues
	expectedRoot, _ := filepath.EvalSymlinks(tempDir)
	actualRoot, _ := filepath.EvalSymlinks(inputs.ProjectRoot)
	assert.Equal(t, expectedRoot, actualRoot)
	assert.Equal(t, "go", inputs.Language)
	expectedAbi, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "abi"))
	actualAbi, _ := filepath.EvalSymlinks(inputs.AbiPath)
	assert.Equal(t, expectedAbi, actualAbi)
	assert.Equal(t, "bindings", inputs.PkgName)
	expectedOut, _ := filepath.EvalSymlinks(filepath.Join(tempDir, "contracts", "evm", "src", "generated"))
	actualOut, _ := filepath.EvalSymlinks(inputs.OutPath)
	assert.Equal(t, expectedOut, actualOut)
}

func TestValidateEvmInputs_ValidEvmInputs(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a valid ABI file
	abiContent := `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`
	abiFile := filepath.Join(tempDir, "test.abi")
	err = os.WriteFile(abiFile, []byte(abiContent), 0600)
	require.NoError(t, err)

	// Test validation with valid inputs (using single file)
	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "go",
		AbiPath:     abiFile,
		PkgName:     "bindings",
		OutPath:     tempDir,
	}

	err = validateEvmInputs(inputs)
	require.NoError(t, err)

	// Test validation with directory containing .abi files
	abiDir := filepath.Join(tempDir, "abi")
	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(abiDir, "contract.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)

	inputs.AbiPath = abiDir
	err = validateEvmInputs(inputs)
	require.NoError(t, err)
}

func TestValidateEvmInputs_InvalidLanguage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test validation with invalid language
	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "typescript", // No longer supported
		AbiPath:     tempDir,
		PkgName:     "bindings",
		OutPath:     tempDir,
	}

	err = validateEvmInputs(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "language")
}

func TestValidateEvmInputs_NonExistentDirectory(t *testing.T) {
	// Test validation with non-existent directory
	inputs := EvmInputs{
		ProjectRoot: "/non/existent/path",
		Language:    "go",
		AbiPath:     "/non/existent/abi",
		PkgName:     "bindings",
		OutPath:     "/non/existent/out",
	}

	err := validateEvmInputs(inputs)
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

	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	// This test will fail because it tries to call the actual bindings.GenerateBindings
	// but it tests the directory processing logic
	err = processEvmAbiDirectory(inputs)
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

	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	// Try to process - the mock ABI content might actually work
	err = processEvmAbiDirectory(inputs)
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

	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	err = processEvmAbiDirectory(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .abi files found")
}

func TestProcessAbiDirectory_PackageNameCollision(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "generate-bindings-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiDir := filepath.Join(tempDir, "abi")
	outDir := filepath.Join(tempDir, "generated")

	err = os.MkdirAll(abiDir, 0755)
	require.NoError(t, err)

	abiContent := `[{"type":"function","name":"test","inputs":[],"outputs":[]}]`

	// "TestContract" -> "test_contract"
	// "test_contract" -> "test_contract"
	err = os.WriteFile(filepath.Join(abiDir, "TestContract.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(abiDir, "test_contract.abi"), []byte(abiContent), 0600)
	require.NoError(t, err)

	inputs := EvmInputs{
		ProjectRoot: tempDir,
		Language:    "go",
		AbiPath:     abiDir,
		PkgName:     "bindings",
		OutPath:     outDir,
	}

	err = processEvmAbiDirectory(inputs)
	fmt.Println(err.Error())
	require.Error(t, err)
	require.Equal(t, err.Error(), "package name collision: multiple contracts would generate the same package name 'test_contract' (contracts are converted to snake_case for package names). Please rename one of your contract files to avoid this conflict")
}

func TestProcessAbiDirectory_NonExistentDirectory(t *testing.T) {
	inputs := EvmInputs{
		ProjectRoot: "/tmp",
		Language:    "go",
		AbiPath:     "/non/existent/abi",
		PkgName:     "bindings",
		OutPath:     "/tmp/out",
	}

	err := processEvmAbiDirectory(inputs)
	require.Error(t, err)
	// For non-existent directory, filepath.Glob returns empty slice, so we get the "no .abi files found" error
	assert.Contains(t, err.Error(), "no .abi files found")
}

// TestGenerateBindings_UnconventionalNaming tests binding generation for contracts
// with unconventional naming patterns to verify correct handling or appropriate errors.
func TestGenerateBindings_UnconventionalNaming(t *testing.T) {
	tests := []struct {
		name           string
		contractABI    string
		pkgName        string
		typeName       string
		shouldFail     bool
		expectedErrMsg string
	}{
		{
			name:     "DollarSignInStructField",
			pkgName:  "dollarsign",
			typeName: "DollarContract",
			contractABI: `[
				{"type":"function","name":"getValue","inputs":[],"outputs":[{"name":"","type":"tuple","components":[{"name":"$name","type":"string"},{"name":"$value","type":"uint256"}]}],"stateMutability":"view"}
			]`,
			shouldFail:     true,
			expectedErrMsg: "invalid name",
		},
		{
			name:     "DollarSignInFunctionName",
			pkgName:  "dollarsign",
			typeName: "DollarFuncContract",
			contractABI: `[
				{"type":"function","name":"$getValue","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}
			]`,
			shouldFail:     true,
			expectedErrMsg: "illegal character",
		},
		{
			name:     "DollarSignInEventName",
			pkgName:  "dollarsign",
			typeName: "DollarEventContract",
			contractABI: `[
				{"type":"event","name":"$Transfer","inputs":[{"name":"from","type":"address","indexed":true}],"anonymous":false}
			]`,
			shouldFail:     true,
			expectedErrMsg: "illegal character",
		},
		{
			name:     "camelCaseContractName",
			pkgName:  "camelcase",
			typeName: "camelCaseContract",
			contractABI: `[
				{"type":"function","name":"getValue","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}
			]`,
			shouldFail: false,
		},
		{
			name:     "snake_case_contract_name",
			pkgName:  "snakecase",
			typeName: "snake_case_contract",
			contractABI: `[
				{"type":"function","name":"get_value","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}
			]`,
			shouldFail: false,
		},
		{
			name:     "snake_case_function_names",
			pkgName:  "snakefunc",
			typeName: "SnakeFuncContract",
			contractABI: `[
				{"type":"function","name":"get_user_balance","inputs":[{"name":"user_address","type":"address"}],"outputs":[{"name":"user_balance","type":"uint256"}],"stateMutability":"view"},
				{"type":"event","name":"balance_updated","inputs":[{"name":"user_address","type":"address","indexed":true},{"name":"new_balance","type":"uint256","indexed":false}],"anonymous":false}
			]`,
			shouldFail: false,
		},
		{
			name:     "ALLCAPS_contract_name",
			pkgName:  "allcaps",
			typeName: "ALLCAPSCONTRACT",
			contractABI: `[
				{"type":"function","name":"GETVALUE","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}
			]`,
			shouldFail: false,
		},
		{
			name:     "MixedCase_With_Underscores",
			pkgName:  "mixedcase",
			typeName: "Mixed_Case_Contract",
			contractABI: `[
				{"type":"function","name":"Get_User_Data","inputs":[{"name":"User_Id","type":"uint256"}],"outputs":[{"name":"","type":"string"}],"stateMutability":"view"}
			]`,
			shouldFail: false,
		},
		{
			name:     "NumericSuffix",
			pkgName:  "numeric",
			typeName: "Contract123",
			contractABI: `[
				{"type":"function","name":"getValue1","inputs":[],"outputs":[{"name":"value1","type":"uint256"}],"stateMutability":"view"},
				{"type":"function","name":"getValue2","inputs":[],"outputs":[{"name":"value2","type":"uint256"}],"stateMutability":"view"}
			]`,
			shouldFail: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "bindings-unconventional-test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			abiFile := filepath.Join(tempDir, tc.typeName+".abi")
			err = os.WriteFile(abiFile, []byte(tc.contractABI), 0600)
			require.NoError(t, err)

			outFile := filepath.Join(tempDir, "bindings.go")
			err = GenerateBindings("", abiFile, tc.pkgName, tc.typeName, outFile)

			if tc.shouldFail {
				require.Error(t, err, "Expected binding generation to fail for %s", tc.name)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")
				}
			} else {
				require.NoError(t, err, "Binding generation should succeed for %s", tc.name)

				content, err := os.ReadFile(outFile)
				require.NoError(t, err)
				assert.NotEmpty(t, content, "Generated bindings should not be empty")

				assert.Contains(t, string(content), fmt.Sprintf("package %s", tc.pkgName))
			}
		})
	}
}
