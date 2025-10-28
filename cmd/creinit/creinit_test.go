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

func GetTemplateFileList() []string {
	return []string{
		"README.md",
		"main.go",
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
			expectTemplateFiles: GetTemplateFileList(),
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
			expectTemplateFiles: GetTemplateFileList(),
		},
		{
			name:             "no flags: prompt project, blank template, prompt workflow",
			projectNameFlag:  "",
			templateIDFlag:   0,
			workflowNameFlag: "",
			rpcURLFlag:       "",
			// "projX" (project), "1" (pick Golang), "2" (pick HelloWorld/blank), "workflow-X" (name)
			// No RPC prompt here since PoR was NOT selected
			mockResponses:       []string{"projX", "1", "2", "workflow-X"},
			expectProjectDirRel: "projX",
			expectWorkflowName:  "workflow-X",
			expectTemplateFiles: GetTemplateFileList(),
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
			expectTemplateFiles: GetTemplateFileList(),
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
			expectTemplateFiles: GetTemplateFileList(),
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
			expectTemplateFiles: GetTemplateFileList(),
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
			expectTemplateFiles: GetTemplateFileList(),
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
		GetTemplateFileList(),
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
