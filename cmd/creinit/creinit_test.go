package creinit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/testutil"
	"github.com/smartcontractkit/dev-platform/internal/testutil/chainsim"
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

func TestInitExecuteFlows(t *testing.T) {
	cases := []struct {
		name                string
		projectNameFlag     string
		projectPathFlag     string
		templateIDFlag      uint32
		workflowNameFlag    string
		mockResponses       []string
		expectProjectDirRel string
		expectWorkflowName  string
		expectTemplateFiles []string
	}{
		{
			name:                "explicit project+path, default template via prompt, custom workflow via prompt",
			projectNameFlag:     "myproj",
			projectPathFlag:     "services",
			templateIDFlag:      0,
			workflowNameFlag:    "",
			mockResponses:       []string{"", "myworkflow"},
			expectProjectDirRel: filepath.Join("services", "myproj"),
			expectWorkflowName:  "myworkflow",
			expectTemplateFiles: GetTemplateFileList(),
		},
		{
			name:                "only project, default template+workflow via prompt",
			projectNameFlag:     "alpha",
			projectPathFlag:     "",
			templateIDFlag:      0,
			workflowNameFlag:    "",
			mockResponses:       []string{"", "default-wf"},
			expectProjectDirRel: "alpha",
			expectWorkflowName:  "default-wf",
			expectTemplateFiles: GetTemplateFileList(),
		},
		{
			name:                "no flags: prompt project, blank template, prompt workflow",
			projectNameFlag:     "",
			projectPathFlag:     "",
			templateIDFlag:      0,
			workflowNameFlag:    "",
			mockResponses:       []string{"projX", "1", "workflow-X"},
			expectProjectDirRel: "projX",
			expectWorkflowName:  "workflow-X",
			expectTemplateFiles: GetTemplateFileList(),
		},
		{
			name:                "workflow-name flag only, default template, no workflow prompt",
			projectNameFlag:     "projFlag",
			projectPathFlag:     "",
			templateIDFlag:      0,
			workflowNameFlag:    "flagged-wf",
			mockResponses:       []string{""},
			expectProjectDirRel: "projFlag",
			expectWorkflowName:  "flagged-wf",
			expectTemplateFiles: GetTemplateFileList(),
		},
		{
			name:                "template-id flag only, no template prompt",
			projectNameFlag:     "tplProj",
			projectPathFlag:     "",
			templateIDFlag:      2,
			workflowNameFlag:    "",
			mockResponses:       []string{"workflow-Tpl"},
			expectProjectDirRel: "tplProj",
			expectWorkflowName:  "workflow-Tpl",
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

			var absProjectPath string
			if tc.projectPathFlag != "" {
				absProjectPath = filepath.Join(tempDir, tc.projectPathFlag)
			}

			inputs := Inputs{
				ProjectPath:  absProjectPath,
				ProjectName:  tc.projectNameFlag,
				TemplateID:   tc.templateIDFlag,
				WorkflowName: tc.workflowNameFlag,
			}

			ctx := sim.NewRuntimeContext()
			mockStdin := testutil.NewMockStdinReader(tc.mockResponses)
			h := newHandler(ctx, mockStdin)

			require.NoError(t, h.ValidateInputs(inputs))
			require.NoError(t, h.Execute(inputs))

			projectRoot := filepath.Join(tempDir, tc.expectProjectDirRel)
			validateInitProjectStructure(t, projectRoot, tc.expectWorkflowName, tc.expectTemplateFiles)
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

	os.Remove(constants.DefaultEnvFileName)

	inputs := Inputs{
		ProjectPath:  "",
		ProjectName:  "",
		TemplateID:   2,
		WorkflowName: "",
	}

	mockStdin := testutil.NewMockStdinReader([]string{"wf-inside-existing-project"})
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
