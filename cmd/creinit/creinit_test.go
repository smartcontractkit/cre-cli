package creinit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

// mockRegistry implements RegistryInterface for testing.
type mockRegistry struct {
	templates   []templaterepo.TemplateSummary
	scaffoldDir string // if set, creates basic files in this dir on scaffold
}

func (m *mockRegistry) ListTemplates(refresh bool) ([]templaterepo.TemplateSummary, error) {
	if len(m.templates) == 0 {
		return nil, fmt.Errorf("no templates available")
	}
	return m.templates, nil
}

func (m *mockRegistry) GetTemplate(name string, refresh bool) (*templaterepo.TemplateSummary, error) {
	for i := range m.templates {
		if m.templates[i].Name == name {
			return &m.templates[i], nil
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}

func (m *mockRegistry) ScaffoldTemplate(tmpl *templaterepo.TemplateSummary, destDir, workflowName string, onProgress func(string)) error {
	// Create a mock workflow directory with basic files
	wfDir := filepath.Join(destDir, workflowName)
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		return err
	}

	var files map[string]string
	if tmpl.Language == "go" {
		files = map[string]string{
			"main.go":    "package main\n",
			"README.md":  "# Test\n",
			"workflow.yaml": "name: test\n",
		}
	} else {
		files = map[string]string{
			"main.ts":    "console.log('hello');\n",
			"README.md":  "# Test\n",
			"workflow.yaml": "name: test\n",
		}
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(wfDir, name), []byte(content), 0600); err != nil {
			return err
		}
	}

	return nil
}

// Test fixtures
var testGoTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "test-go",
		Title:       "Test Go Template",
		Description: "A test Go template",
		Language:    "go",
		Category:    "test",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia"},
	},
	Path: "building-blocks/test/test-go",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testTSTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "test-ts",
		Title:       "Test TypeScript Template",
		Description: "A test TypeScript template",
		Language:    "typescript",
		Category:    "test",
		Author:      "Test",
		License:     "MIT",
	},
	Path: "building-blocks/test/test-ts",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testStarterTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "starter-template",
		Name:        "starter-go",
		Title:       "Starter Go Template",
		Description: "A starter Go template",
		Language:    "go",
		Category:    "test",
		Author:      "Test",
		License:     "MIT",
	},
	Path: "starter-templates/test/starter-go",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testMultiNetworkTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "test-multichain",
		Title:       "Test Multi-Chain Template",
		Description: "A template requiring multiple chains",
		Language:    "go",
		Category:    "test",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia", "ethereum-mainnet"},
	},
	Path: "building-blocks/test/test-multichain",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		templates: []templaterepo.TemplateSummary{
			testGoTemplate,
			testTSTemplate,
			testStarterTemplate,
			testMultiNetworkTemplate,
		},
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

func TestInitExecuteFlows(t *testing.T) {
	cases := []struct {
		name                string
		projectNameFlag     string
		templateNameFlag    string
		workflowNameFlag    string
		rpcURLs             map[string]string
		expectProjectDirRel string
		expectWorkflowName  string
		expectTemplateFiles []string
	}{
		{
			name:                "Go template with all flags",
			projectNameFlag:     "myproj",
			templateNameFlag:    "test-go",
			workflowNameFlag:    "myworkflow",
			rpcURLs:             map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
			expectProjectDirRel: "myproj",
			expectWorkflowName:  "myworkflow",
			expectTemplateFiles: GetTemplateFileListGo(),
		},
		{
			name:                "TypeScript template with all flags",
			projectNameFlag:     "tsProj",
			templateNameFlag:    "test-ts",
			workflowNameFlag:    "ts-workflow",
			expectProjectDirRel: "tsProj",
			expectWorkflowName:  "ts-workflow",
			expectTemplateFiles: GetTemplateFileListTS(),
		},
		{
			name:                "Starter template with all flags",
			projectNameFlag:     "starterProj",
			templateNameFlag:    "starter-go",
			workflowNameFlag:    "starter-wf",
			expectProjectDirRel: "starterProj",
			expectWorkflowName:  "starter-wf",
			expectTemplateFiles: GetTemplateFileListGo(),
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
				TemplateName: tc.templateNameFlag,
				WorkflowName: tc.workflowNameFlag,
				RpcURLs:      tc.rpcURLs,
			}

			ctx := sim.NewRuntimeContext()
			h := newHandlerWithRegistry(ctx, newMockRegistry())

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

	_ = os.Remove(constants.DefaultEnvFileName)

	inputs := Inputs{
		ProjectName:  "",
		TemplateName: "test-go",
		WorkflowName: "wf-inside-existing-project",
		RpcURLs:      map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())

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
		TemplateName: "test-ts",
		WorkflowName: "ts-workflow-01",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())

	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "tsProj")

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.DirExists(t, filepath.Join(projectRoot, "ts-workflow-01"))

	// go.mod should NOT exist for TS templates
	modPath := filepath.Join(projectRoot, "go.mod")
	_, err = os.Stat(modPath)
	require.Truef(t, os.IsNotExist(err), "go.mod should NOT exist for TypeScript templates (found at %s)", modPath)
}

func TestInitWithRpcUrlFlags(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:  "rpcProj",
		TemplateName: "test-multichain",
		WorkflowName: "rpc-workflow",
		RpcURLs: map[string]string{
			"ethereum-testnet-sepolia": "https://sepolia.example.com",
			"ethereum-mainnet":        "https://mainnet.example.com",
		},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "rpcProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)
	require.Contains(t, content, "ethereum-testnet-sepolia")
	require.Contains(t, content, "https://sepolia.example.com")
	require.Contains(t, content, "ethereum-mainnet")
	require.Contains(t, content, "https://mainnet.example.com")
}

func TestInitNoNetworksFallsBackToDefault(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// testTSTemplate has no Networks field
	inputs := Inputs{
		ProjectName:  "defaultProj",
		TemplateName: "test-ts",
		WorkflowName: "default-wf",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "defaultProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)
	require.Contains(t, content, "ethereum-testnet-sepolia")
	require.Contains(t, content, constants.DefaultEthSepoliaRpcUrl)
}

func TestTemplateNotFound(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:  "proj",
		TemplateName: "nonexistent-template",
		WorkflowName: "wf",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())

	require.NoError(t, h.ValidateInputs(inputs))
	err = h.Execute(inputs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
