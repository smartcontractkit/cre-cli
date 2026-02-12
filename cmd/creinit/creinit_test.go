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
		"workflow.yaml",
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

	testFile := filepath.Join(workflowDir, "main.test.ts")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Logf("Skipping TS tests: no main.test.ts in %s", workflowDir)
		return
	}

	t.Logf("Running TypeScript tests in %s", workflowDir)

	// Install dependencies using bun install --cwd (as instructed by cre init)
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

	// Check if there's a go.mod or any .go test files
	hasGoTests := false
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Logf("Skipping Go tests: cannot read %s", workflowDir)
		return
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == "_test.go" {
			hasGoTests = true
			break
		}
	}

	if !hasGoTests {
		t.Logf("Skipping Go tests: no *_test.go files in %s", workflowDir)
		return
	}

	t.Logf("Running Go tests in %s", workflowDir)

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = workflowDir
	testOutput, err := testCmd.CombinedOutput()
	require.NoError(t, err, "go test failed in %s:\n%s", workflowDir, string(testOutput))
	t.Logf("go test passed:\n%s", string(testOutput))
}

func TestInitExecuteFlows(t *testing.T) {
	cases := []struct {
		name                string
		projectNameFlag     string
		templateIDFlag      uint32
		workflowNameFlag    string
		rpcURLFlag          string
		mockResponses       []string
		expectProjectDirRel string
		expectWorkflowName  string
		expectTemplateFiles []string
		language            string // "go" or "typescript"
	}{
		{
			name:             "explicit project, default template via prompt, custom workflow via prompt",
			projectNameFlag:  "myproj",
			templateIDFlag:   0,
			workflowNameFlag: "",
			rpcURLFlag:       "",
			// "" (language default -> Golang), "" (workflow default -> PoR), "" (RPC URL accept default), "myworkflow"
			mockResponses:       []string{"", "", "", "myworkflow"},
			expectProjectDirRel: "myproj",
			expectWorkflowName:  "myworkflow",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:             "only project, default template+workflow via prompt",
			projectNameFlag:  "alpha",
			templateIDFlag:   0,
			workflowNameFlag: "",
			rpcURLFlag:       "",
			// defaults to PoR -> include extra "" for RPC URL
			mockResponses:       []string{"", "", "", "default-wf"},
			expectProjectDirRel: "alpha",
			expectWorkflowName:  "default-wf",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:             "no flags: prompt project, blank template, prompt workflow",
			projectNameFlag:  "",
			templateIDFlag:   0,
			workflowNameFlag: "",
			rpcURLFlag:       "",
			// "projX" (project), "1" (pick Golang), "2" (pick HelloWorld/blank), "workflow-X" (name)
			// No RPC prompt here since PoR was NOT selected
			mockResponses:       []string{"projX", "1", "2", "", "workflow-X"},
			expectProjectDirRel: "projX",
			expectWorkflowName:  "workflow-X",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:             "workflow-name flag only, default template, no workflow prompt",
			projectNameFlag:  "projFlag",
			templateIDFlag:   0,
			workflowNameFlag: "flagged-wf",
			rpcURLFlag:       "",
			// defaults to PoR → include RPC URL accept
			mockResponses:       []string{"", "", ""},
			expectProjectDirRel: "projFlag",
			expectWorkflowName:  "flagged-wf",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:                "template-id flag only, no template prompt",
			projectNameFlag:     "tplProj",
			templateIDFlag:      2,
			workflowNameFlag:    "",
			rpcURLFlag:          "",
			mockResponses:       []string{"workflow-Tpl"},
			expectProjectDirRel: "tplProj",
			expectWorkflowName:  "workflow-Tpl",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:             "PoR template via flag with rpc-url provided (skips RPC prompt)",
			projectNameFlag:  "porWithFlag",
			templateIDFlag:   1, // Golang PoR
			workflowNameFlag: "",
			rpcURLFlag:       "https://sepolia.example/rpc",
			// Only needs a workflow name prompt
			mockResponses:       []string{"por-wf-01"},
			expectProjectDirRel: "porWithFlag",
			expectWorkflowName:  "por-wf-01",
			expectTemplateFiles: GetTemplateFileListGo(),
			language:            "go",
		},
		{
			name:             "TS template with rpc-url provided (flag ignored; no RPC prompt needed)",
			projectNameFlag:  "tsWithRpcFlag",
			templateIDFlag:   3, // TypeScript HelloWorld
			workflowNameFlag: "",
			rpcURLFlag:       "https://sepolia.example/rpc",
			// Just the workflow name prompt
			mockResponses:       []string{"ts-wf-flag"},
			expectProjectDirRel: "tsWithRpcFlag",
			expectWorkflowName:  "ts-wf-flag",
			expectTemplateFiles: GetTemplateFileListTS(),
			language:            "typescript",
		},
		{
			name:             "TS PoR template",
			projectNameFlag:  "tsPorProj",
			templateIDFlag:   4, // TypeScript PoR
			workflowNameFlag: "ts-por-wf",
			rpcURLFlag:       "https://sepolia.example/rpc",
			mockResponses:       []string{},
			expectProjectDirRel: "tsPorProj",
			expectWorkflowName:  "ts-por-wf",
			expectTemplateFiles: GetTemplateFileListTS(),
			language:            "typescript",
		},
		{
			name:             "TS Confidential HTTP template",
			projectNameFlag:  "tsConfHTTP",
			templateIDFlag:   5, // TypeScript Confidential HTTP
			workflowNameFlag: "ts-confhttp-wf",
			rpcURLFlag:       "",
			mockResponses:       []string{},
			expectProjectDirRel: "tsConfHTTP",
			expectWorkflowName:  "ts-confhttp-wf",
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
			mockStdin := testutil.NewMockStdinReader(tc.mockResponses)
			h := newHandler(ctx, mockStdin)

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
		TemplateID:   2,
		WorkflowName: "",
	}

	mockStdin := testutil.NewMockStdinReader([]string{"wf-inside-existing-project", ""})
	h := newHandler(sim.NewRuntimeContext(), mockStdin)

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
		WorkflowName: "",
	}

	// Ensure workflow name meets 10-char minimum
	mockStdin := testutil.NewMockStdinReader([]string{"ts-workflow-01"})
	h := newHandler(sim.NewRuntimeContext(), mockStdin)

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
		TemplateID:   3,
		WorkflowName: "",
	}

	mockStdin := testutil.NewMockStdinReader([]string{"ts-wf-existing"})
	h := newHandler(sim.NewRuntimeContext(), mockStdin)

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
