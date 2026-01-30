package creinit

import (
	"fmt"
	"os"
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
			// NOTE: We deliberately don't assert Go/TS scaffolding here because the
			// template chosen by prompt could vary; dedicated tests below cover both paths.
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
