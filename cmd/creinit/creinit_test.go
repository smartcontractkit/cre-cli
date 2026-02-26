package creinit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func GetTemplateFileListGo() []string {
	return []string{
		"README.md",
		"main.go",
		"workflow.go",
		"workflow.yaml",
		"workflow_test.go",
	}
}

func GetTemplateFileListTS() []string {
	return []string{
		"README.md",
		"main.ts",
		"workflow.yaml",
	}
}

func validateInitProjectStructure(t *testing.T, projectRoot, workflowName string, expectedFiles []string) {
	require.FileExists(
		t,
		filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName),
		"project.yaml must exist in project root",
	)
	require.FileExists(
		t,
		filepath.Join(projectRoot, constants.DefaultEnvFileName),
		".env must exist in project root",
	)

	wfDir := filepath.Join(projectRoot, workflowName)
	require.DirExists(t, wfDir, "workflow directory should exist")

	for _, f := range expectedFiles {
		require.FileExists(
			t,
			filepath.Join(wfDir, f),
			fmt.Sprintf("template file %q missing in workflow dir", f),
		)
	}
}

func validateGoScaffoldAbsent(t *testing.T, projectRoot string) {
	t.Helper()
	// go.mod should NOT exist
	modPath := filepath.Join(projectRoot, "go.mod")
	_, err := os.Stat(modPath)
	require.Truef(t, os.IsNotExist(err), "go.mod should NOT exist for TypeScript templates (found at %s)", modPath)

	// contracts/ dir should NOT exist at project root
	contractsDir := filepath.Join(projectRoot, "contracts")
	requireNoDirExists(t, contractsDir)
}

func requireNoDirExists(t *testing.T, dirPath string) {
	t.Helper()
	fi, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return // good: no directory
	}
	require.NoError(t, err, "unexpected error stating %s", dirPath)
	require.Falsef(t, fi.IsDir(), "directory %s should NOT exist", dirPath)
}

func hashDirectoryFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	hashes := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	require.NoError(t, err)
	return hashes
}

func validateGeneratedBindingsStable(t *testing.T, projectRoot, workflowName, language string) {
	t.Helper()

	abiDir := filepath.Join(projectRoot, "contracts", "evm", "src", "abi")

	var generatedDir string
	switch language {
	case "go":
		generatedDir = filepath.Join(projectRoot, "contracts", "evm", "src", "generated")
	case "typescript":
		generatedDir = filepath.Join(projectRoot, "contracts", "evm", "ts", "generated")
	default:
		return
	}

	if _, err := os.Stat(generatedDir); os.IsNotExist(err) {
		return
	}

	beforeHashes := hashDirectoryFiles(t, generatedDir)
	require.NotEmpty(t, beforeHashes, "generated directory should not be empty")

	abiFiles, err := filepath.Glob(filepath.Join(abiDir, "*.abi"))
	require.NoError(t, err)
	require.NotEmpty(t, abiFiles, "abi directory should contain *.abi files")

	switch language {
	case "go":
		for _, abiFile := range abiFiles {
			contractName := strings.TrimSuffix(filepath.Base(abiFile), ".abi")
			entries, readErr := os.ReadDir(generatedDir)
			require.NoError(t, readErr)

			found := false
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				goFile := filepath.Join(generatedDir, entry.Name(), contractName+".go")
				if _, statErr := os.Stat(goFile); statErr == nil {
					err = bindings.GenerateBindings("", abiFile, entry.Name(), contractName, goFile)
					require.NoError(t, err, "failed to regenerate Go bindings for %s", contractName)
					found = true
					break
				}
			}
			require.True(t, found, "no matching generated directory found for contract %s", contractName)
		}

	case "typescript":
		var generatedContracts []string
		for _, abiFile := range abiFiles {
			ext := filepath.Ext(abiFile)
			contractName := strings.TrimSuffix(filepath.Base(abiFile), ext)
			outFile := filepath.Join(generatedDir, contractName+".ts")
			err = bindings.GenerateBindingsTS(abiFile, contractName, outFile)
			require.NoError(t, err, "failed to regenerate TS bindings for %s", contractName)
			generatedContracts = append(generatedContracts, contractName)
		}
		// Regenerate barrel index.ts
		var indexContent string
		indexContent += "// Code generated — DO NOT EDIT.\n"
		for _, name := range generatedContracts {
			indexContent += fmt.Sprintf("export * from './%s'\n", name)
			indexContent += fmt.Sprintf("export * from './%s_mock'\n", name)
		}
		indexPath := filepath.Join(generatedDir, "index.ts")
		require.NoError(t, os.WriteFile(indexPath, []byte(indexContent), 0o600))
	}

	afterHashes := hashDirectoryFiles(t, generatedDir)

	require.Equal(t, len(beforeHashes), len(afterHashes), "number of generated files changed after regeneration")
	for file, beforeHash := range beforeHashes {
		afterHash, exists := afterHashes[file]
		require.True(t, exists, "generated file %s disappeared after regeneration", file)
		require.Equal(t, beforeHash, afterHash, "generated file %s changed after regeneration — template is stale", file)
	}
}

// runLanguageSpecificTests runs the appropriate test suite based on the language field.
// For TypeScript: runs bun install and bun test in the workflow directory.
// For Go: runs go test ./... in the workflow directory.
func runLanguageSpecificTests(t *testing.T, workflowDir, language string) {
	t.Helper()

	switch language {
	case "typescript":
		runTypescriptTests(t, workflowDir)
	case "go":
		runGoTests(t, workflowDir)
	default:
		t.Logf("Unknown language %q, skipping tests", language)
	}
}

// runTypescriptTests executes TypeScript tests using bun.
// Follows the cre init instructions: bun install --cwd <dir> then bun test in that directory.
// For projects with contracts (e.g. TS PoR), also installs contracts dependencies so generated
// bindings can resolve @chainlink/cre-sdk.
func runTypescriptTests(t *testing.T, workflowDir string) {
	t.Helper()

	t.Logf("Running TypeScript tests in %s", workflowDir)
	installCmd := exec.Command("bun", "install", "--cwd", workflowDir, "--ignore-scripts")
	installOutput, err := installCmd.CombinedOutput()
	require.NoError(t, err, "bun install failed in %s:\n%s", workflowDir, string(installOutput))
	t.Logf("bun install succeeded")

	// Install contracts dependencies when contracts/package.json exists (TS PoR template)
	projectRoot := filepath.Dir(workflowDir)
	contractsPkg := filepath.Join(projectRoot, "contracts", "package.json")
	if _, err := os.Stat(contractsPkg); err == nil {
		contractsDir := filepath.Join(projectRoot, "contracts")
		installCmd := exec.Command("bun", "install", "--cwd", contractsDir, "--ignore-scripts")
		installOutput, err := installCmd.CombinedOutput()
		require.NoError(t, err, "bun install failed in %s:\n%s", contractsDir, string(installOutput))
		t.Logf("bun install in contracts succeeded")
	}

	testCmd := exec.Command("bun", "test")
	testCmd.Dir = workflowDir
	testOutput, err := testCmd.CombinedOutput()
	require.NoError(t, err, "bun test failed in %s:\n%s", workflowDir, string(testOutput))
	t.Logf("bun test passed:\n%s", string(testOutput))
}

// runGoTests executes Go tests in the workflow directory.
func runGoTests(t *testing.T, workflowDir string) {
	t.Helper()

	t.Logf("Running Go tests in %s", workflowDir)

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = workflowDir
	testOutput, err := testCmd.CombinedOutput()
	require.NoError(t, err, "go test failed in %s:\n%s", workflowDir, string(testOutput))
	t.Logf("go test passed:\n%s", string(testOutput))
}

func TestInitExecuteFlows(t *testing.T) {
	// All inputs are provided via flags to avoid interactive prompts
	cases := []struct {
		name                string
		projectNameFlag     string
		templateIDFlag      uint32
		workflowNameFlag    string
		rpcURLFlag          string
		expectProjectDirRel string
		expectWorkflowName  string
		expectTemplateFiles []string
		language            string // "go" or "typescript"
	}{
		{
			name:                "Go PoR template with all flags",
			projectNameFlag:     "myproj",
			templateIDFlag:      1, // Golang PoR
			workflowNameFlag:    "myworkflow",
			rpcURLFlag:          "https://sepolia.example/rpc",
			expectProjectDirRel: "myproj",
			expectWorkflowName:  "myworkflow",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "Go HelloWorld template with all flags",
			projectNameFlag:     "alpha",
			templateIDFlag:      2, // Golang HelloWorld
			workflowNameFlag:    "default-wf",
			rpcURLFlag:          "",
			expectProjectDirRel: "alpha",
			expectWorkflowName:  "default-wf",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "Go HelloWorld with different project name",
			projectNameFlag:     "projX",
			templateIDFlag:      2, // Golang HelloWorld
			workflowNameFlag:    "workflow-X",
			rpcURLFlag:          "",
			expectProjectDirRel: "projX",
			expectWorkflowName:  "workflow-X",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "Go PoR with workflow flag",
			projectNameFlag:     "projFlag",
			templateIDFlag:      1, // Golang PoR
			workflowNameFlag:    "flagged-wf",
			rpcURLFlag:          "https://sepolia.example/rpc",
			expectProjectDirRel: "projFlag",
			expectWorkflowName:  "flagged-wf",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "Go HelloWorld template by ID",
			projectNameFlag:     "tplProj",
			templateIDFlag:      2, // Golang HelloWorld
			workflowNameFlag:    "workflow-Tpl",
			rpcURLFlag:          "",
			expectProjectDirRel: "tplProj",
			expectWorkflowName:  "workflow-Tpl",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "Go PoR template with rpc-url",
			projectNameFlag:     "porWithFlag",
			templateIDFlag:      1, // Golang PoR
			workflowNameFlag:    "por-wf-01",
			rpcURLFlag:          "https://sepolia.example/rpc",
			expectProjectDirRel: "porWithFlag",
			expectWorkflowName:  "por-wf-01",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "TS HelloWorld template with rpc-url (ignored)",
			projectNameFlag:     "tsWithRpcFlag",
			templateIDFlag:      3, // TypeScript HelloWorld
			workflowNameFlag:    "ts-wf-flag",
			rpcURLFlag:          "https://sepolia.example/rpc",
			expectProjectDirRel: "tsWithRpcFlag",
			expectWorkflowName:  "ts-wf-flag",
			expectTemplateFiles: GetTemplateFileListTS(),
			language:            "typescript",
		},
		{
			name:                "TS PoR template",
			projectNameFlag:     "tsPorProj",
			templateIDFlag:      4, // TypeScript PoR
			workflowNameFlag:    "ts-por-wf",
			rpcURLFlag:          "https://sepolia.example/rpc",
			expectProjectDirRel: "tsPorProj",
			expectWorkflowName:  "ts-por-wf",
			expectTemplateFiles: GetTemplateFileListTS(),
			language:            "typescript",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sim := chainsim.NewSimulatedEnvironment(t)
			defer sim.Close()

			tempDir := t.TempDir()
			restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
			require.NoError(t, err)
			defer restoreCwd()

			inputs := Inputs{
				ProjectName:  tc.projectNameFlag,
				TemplateID:   tc.templateIDFlag,
				WorkflowName: tc.workflowNameFlag,
				RPCUrl:       tc.rpcURLFlag,
			}

			ctx := sim.NewRuntimeContext()
			h := newHandler(ctx)

			require.NoError(t, h.ValidateInputs(inputs))
			require.NoError(t, h.Execute(inputs))

			projectRoot := filepath.Join(tempDir, tc.expectProjectDirRel)
			validateInitProjectStructure(t, projectRoot, tc.expectWorkflowName, tc.expectTemplateFiles)

			runLanguageSpecificTests(t, filepath.Join(projectRoot, tc.expectWorkflowName), tc.language)
			validateGeneratedBindingsStable(t, projectRoot, tc.expectWorkflowName, tc.language)
		})
	}
}

func TestInsideExistingProjectAddsWorkflow(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	require.NoError(t, os.WriteFile(
		constants.DefaultProjectSettingsFileName,
		[]byte("name: existing"), 0600,
	))

	_ = os.Remove(constants.DefaultEnvFileName)

	inputs := Inputs{
		ProjectName:  "",
		TemplateID:   2, // Golang HelloWorld
		WorkflowName: "wf-inside-existing-project",
	}

	h := newHandler(sim.NewRuntimeContext())

	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	require.FileExists(t, constants.DefaultProjectSettingsFileName)
	require.FileExists(t, constants.DefaultEnvFileName)

	validateInitProjectStructure(
		t,
		".",
		"wf-inside-existing-project",
		GetTemplateFileListGo(),
	)
}

func TestInitWithTypescriptTemplateSkipsGoScaffold(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:  "tsProj",
		TemplateID:   3, // TypeScript template
		WorkflowName: "ts-workflow-01",
	}

	h := newHandler(sim.NewRuntimeContext())

	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "tsProj")

	// Generic project assets
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.DirExists(t, filepath.Join(projectRoot, "ts-workflow-01"))

	// TS should NOT create Go artifacts
	validateGoScaffoldAbsent(t, projectRoot)
}

func TestInsideExistingProjectAddsTypescriptWorkflowSkipsGoScaffold(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Simulate an existing project
	require.NoError(t, os.WriteFile(
		constants.DefaultProjectSettingsFileName,
		[]byte("name: existing"), 0600,
	))
	_ = os.Remove(constants.DefaultEnvFileName)

	inputs := Inputs{
		ProjectName:  "",
		TemplateID:   3, // TypeScript HelloWorld
		WorkflowName: "ts-wf-existing",
	}

	h := newHandler(sim.NewRuntimeContext())

	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	require.FileExists(t, constants.DefaultProjectSettingsFileName)
	require.FileExists(t, constants.DefaultEnvFileName)
	require.DirExists(t, "ts-wf-existing")

	// Ensure Go bits are not introduced
	validateGoScaffoldAbsent(t, ".")
}

func TestGetWorkflowTemplateByIDAndTitle(t *testing.T) {
	tpl, lang, err := (&handler{}).getWorkflowTemplateByID(3)
	require.NoError(t, err)
	require.Equal(t, uint32(3), tpl.ID)
	require.Equal(t, lang.Title, "Typescript")
	require.NotEmpty(t, tpl.Title)

	_, _, err = (&handler{}).getWorkflowTemplateByID(9999)
	require.Error(t, err)

	title := tpl.Title
	lang, langErr := (&handler{}).getLanguageTemplateByTitle("Typescript")
	tplByTitle, err := (&handler{}).getWorkflowTemplateByTitle(title, lang.Workflows)
	require.NoError(t, err)
	require.NoError(t, langErr)
	require.Equal(t, tpl.ID, tplByTitle.ID)

	_, err = (&handler{}).getWorkflowTemplateByTitle("this-title-should-not-exist", lang.Workflows)
	require.Error(t, err)
}
