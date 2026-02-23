package creinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
func runTypescriptTests(t *testing.T, workflowDir string) {
	t.Helper()

	t.Logf("Running TypeScript tests in %s", workflowDir)
	installCmd := exec.Command("bun", "install", "--cwd", workflowDir, "--ignore-scripts")
	installOutput, err := installCmd.CombinedOutput()
	require.NoError(t, err, "bun install failed in %s:\n%s", workflowDir, string(installOutput))
	t.Logf("bun install succeeded")

	// Run tests
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
