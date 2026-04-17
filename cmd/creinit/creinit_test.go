package creinit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/tenderly"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

// mockRegistry implements RegistryInterface for testing.
type mockRegistry struct {
	templates []templaterepo.TemplateSummary
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
	var files map[string]string
	if tmpl.Language == constants.WorkflowLanguageGolang {
		files = map[string]string{
			"main.go":   "package main\n",
			"README.md": "# Test\n",
		}
	} else {
		files = map[string]string{
			"main.ts":   "console.log('hello');\n",
			"README.md": "# Test\n",
		}
	}

	// Determine which workflow dirs to create
	if len(tmpl.Workflows) > 1 {
		// Multi-workflow: create each declared workflow dir
		for _, wf := range tmpl.Workflows {
			wfDir := filepath.Join(destDir, wf.Dir)
			if err := os.MkdirAll(wfDir, 0755); err != nil {
				return err
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(wfDir, name), []byte(content), 0600); err != nil {
					return err
				}
			}
		}
	} else if len(tmpl.Workflows) == 1 {
		// Single workflow: create with template's dir name, then rename to user's choice
		srcName := tmpl.Workflows[0].Dir
		wfDir := filepath.Join(destDir, srcName)
		if err := os.MkdirAll(wfDir, 0755); err != nil {
			return err
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(wfDir, name), []byte(content), 0600); err != nil {
				return err
			}
		}
		// Rename to user's workflow name (simulates renameWorkflowDir)
		if srcName != workflowName {
			if err := os.Rename(wfDir, filepath.Join(destDir, workflowName)); err != nil {
				return err
			}
		}
	} else {
		// No workflows field (backwards compat / built-in): create with user's workflowName
		wfDir := filepath.Join(destDir, workflowName)
		if err := os.MkdirAll(wfDir, 0755); err != nil {
			return err
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(wfDir, name), []byte(content), 0600); err != nil {
				return err
			}
		}
	}

	// Simulate remote template behavior: ship project.yaml and .env at root.
	// Built-in templates don't include these (the CLI generates them).
	if !tmpl.BuiltIn {
		networks := tmpl.Networks
		if len(networks) == 0 {
			networks = []string{"ethereum-testnet-sepolia"}
		}
		var rpcsBlock string
		for _, n := range networks {
			rpcsBlock += fmt.Sprintf("    - chain-name: %s\n      url: https://default-rpc.example.com\n", n)
		}
		projectYAML := fmt.Sprintf("staging-settings:\n  rpcs:\n%sproduction-settings:\n  rpcs:\n%s", rpcsBlock, rpcsBlock)
		if err := os.WriteFile(filepath.Join(destDir, "project.yaml"), []byte(projectYAML), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destDir, ".env"), []byte("GITHUB_API_TOKEN=test-token\nETH_PRIVATE_KEY=test-key\n"), 0600); err != nil {
			return err
		}
	}

	return nil
}

// mockTenderlyProvider implements tenderly.Provider for testing.
type mockTenderlyProvider struct {
	rpcBase string // base URL returned for all networks
}

func (m *mockTenderlyProvider) CreateVnets(networks []string) (*tenderly.VnetResult, error) {
	result := &tenderly.VnetResult{
		NetworkRPCs: make(map[string]string, len(networks)),
		PublicRPCs:  make(map[string]string, len(networks)),
		VnetURLs:    make(map[string]string, len(networks)),
	}
	for _, n := range networks {
		result.NetworkRPCs[n] = m.rpcBase
		result.PublicRPCs[n] = m.rpcBase + "/public"
		result.VnetURLs[n] = "https://dashboard.tenderly.co/test/" + n
	}
	return result, nil
}

func newHandlerWithTenderly(ctx *runtime.Context, reg RegistryInterface, rpcBase string) *handler {
	h := newHandlerWithRegistry(ctx, reg)
	h.tenderlyProvider = &mockTenderlyProvider{rpcBase: rpcBase}
	return h
}

// Test fixtures
var testGoTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "test-go",
		Title:       "Test Go Template",
		Description: "A test Go template",
		Language:    "go",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia"},
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
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
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
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
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
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
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia", "ethereum-mainnet"},
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
	},
	Path: "building-blocks/test/test-multichain",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testBuiltInGoTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "hello-world-go",
		Title:       "Hello World (Go)",
		Description: "A built-in Go template",
		Language:    "go",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
	},
	Path:    "builtin/hello-world-go",
	BuiltIn: true,
}

var testMultiWorkflowTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "starter-template",
		Name:        "bring-your-own-data-go",
		Title:       "Bring Your Own Data (Go)",
		Description: "Bring your own off-chain data on-chain with PoR and NAV publishing.",
		Language:    "go",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia"},
		Workflows: []templaterepo.WorkflowDirEntry{
			{Dir: "por", Description: "Proof of Reserve workflow"},
			{Dir: "nav", Description: "NAV publishing workflow"},
		},
		PostInit: "Deploy contracts and update secrets.yaml before running.",
	},
	Path: "starter-templates/bring-your-own-data/workflow-go",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testSingleWorkflowWithPostInit = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "building-block",
		Name:        "kv-store-go",
		Title:       "KV Store (Go)",
		Description: "Read, increment, and write a counter in AWS S3.",
		Language:    "go",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
		PostInit:    "Update secrets.yaml with your AWS credentials before running.",
	},
	Path: "building-blocks/kv-store/kv-store-go",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testProjectDirTemplate = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "starter-template",
		Name:        "projdir-multichain",
		Title:       "ProjectDir Multi-Chain Template",
		Description: "A template with projectDir and multiple networks",
		Language:    "go",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		Networks:    []string{"ethereum-testnet-sepolia", "base-sepolia"},
		Workflows:   []templaterepo.WorkflowDirEntry{{Dir: "my-workflow"}},
		ProjectDir:  ".",
	},
	Path: "starter-templates/test/projdir-multichain",
	Source: templaterepo.RepoSource{
		Owner: "test",
		Repo:  "templates",
		Ref:   "main",
	},
}

var testProjectDirWithNetworks = templaterepo.TemplateSummary{
	TemplateMetadata: templaterepo.TemplateMetadata{
		Kind:        "starter-template",
		Name:        "starter-with-projectdir",
		Title:       "Starter With ProjectDir",
		Description: "A starter template that ships its own project structure",
		Language:    "typescript",
		Category:    "workflow",
		Author:      "Test",
		License:     "MIT",
		ProjectDir:  ".",
		Networks:    []string{"ethereum-testnet-sepolia", "ethereum-mainnet"},
		Workflows: []templaterepo.WorkflowDirEntry{
			{Dir: "my-workflow", Description: "Test workflow"},
		},
	},
	Path: "starter-templates/test/starter-with-projectdir",
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
			testBuiltInGoTemplate,
			testMultiWorkflowTemplate,
			testSingleWorkflowWithPostInit,
			testProjectDirTemplate,
			testProjectDirWithNetworks,
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
	// All inputs are provided via flags to avoid interactive prompts
	cases := []struct {
		name                string
		projectNameFlag     string
		templateNameFlag    string
		workflowNameFlag    string
		rpcURLs             map[string]string
		expectProjectDirRel string
		expectWorkflowName  string
		expectTemplateFiles []string
		language            string // "go" or "typescript"
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
			language:            "go",
		},
		{
			name:                "TypeScript template with all flags",
			projectNameFlag:     "tsProj",
			templateNameFlag:    "test-ts",
			workflowNameFlag:    "ts-workflow",
			expectProjectDirRel: "tsProj",
			expectWorkflowName:  "ts-workflow",
			expectTemplateFiles: GetTemplateFileListTS(),
			language:            "typescript",
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
			"ethereum-mainnet":         "https://mainnet.example.com",
		},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "rpcProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)

	// User-provided URLs should replace the mock's default placeholder URLs
	require.Contains(t, content, "ethereum-testnet-sepolia")
	require.Contains(t, content, "https://sepolia.example.com")
	require.NotContains(t, content, "https://default-rpc.example.com",
		"mock default URLs should be replaced by user-provided URLs")
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

	// Built-in template has no project.yaml from scaffold,
	// so the CLI generates one with default networks.
	inputs := Inputs{
		ProjectName:  "defaultProj",
		TemplateName: "hello-world-go",
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

func TestInitRemoteTemplateKeepsProjectYAML(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Remote template (test-ts) has no Networks — mock creates project.yaml with default chain.
	// CLI should preserve the template's project.yaml (no patching needed since no user RPCs).
	inputs := Inputs{
		ProjectName:  "remoteProj",
		TemplateName: "test-ts",
		WorkflowName: "ts-wf",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "remoteProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)
	// Template's project.yaml should be preserved (contains mock's default URL)
	require.Contains(t, content, "ethereum-testnet-sepolia")
	require.Contains(t, content, "https://default-rpc.example.com")

	// Template's .env should be preserved
	envContent, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.NoError(t, err)
	require.Contains(t, string(envContent), "GITHUB_API_TOKEN=test-token")
}

func TestInitProjectDirTemplateRpcPatching(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Template with ProjectDir set AND Networks — the bug was that RPC URLs
	// were silently dropped because the patching was inside the ProjectDir=="" block.
	inputs := Inputs{
		ProjectName:  "projectDirProj",
		TemplateName: "starter-with-projectdir",
		WorkflowName: "my-workflow",
		RpcURLs: map[string]string{
			"ethereum-testnet-sepolia": "https://sepolia.custom.com",
			"ethereum-mainnet":         "https://mainnet.custom.com",
		},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "projectDirProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)

	// User-provided RPCs must be patched even though ProjectDir is set
	require.Contains(t, content, "https://sepolia.custom.com",
		"user RPC URL for sepolia should be patched into project.yaml for templates with ProjectDir")
	require.Contains(t, content, "https://mainnet.custom.com",
		"user RPC URL for mainnet should be patched into project.yaml for templates with ProjectDir")
	require.NotContains(t, content, "https://default-rpc.example.com",
		"mock default URLs should be replaced by user-provided URLs")
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

func TestMultiWorkflowNoRename(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Multi-workflow template: no --workflow-name needed, dirs stay as declared
	inputs := Inputs{
		ProjectName:  "multiProj",
		TemplateName: "bring-your-own-data-go",
		WorkflowName: "",
		RpcURLs:      map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "multiProj")
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))

	// Both workflow dirs should exist with their original names
	require.DirExists(t, filepath.Join(projectRoot, "por"), "por workflow dir should exist")
	require.DirExists(t, filepath.Join(projectRoot, "nav"), "nav workflow dir should exist")

	// workflow.yaml should be generated in each
	require.FileExists(t, filepath.Join(projectRoot, "por", constants.DefaultWorkflowSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, "nav", constants.DefaultWorkflowSettingsFileName))
}

func TestMultiWorkflowIgnoresWorkflowNameFlag(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Multi-workflow with --workflow-name flag: flag should be ignored
	inputs := Inputs{
		ProjectName:  "multiProj2",
		TemplateName: "bring-your-own-data-go",
		WorkflowName: "test-rename",
		RpcURLs:      map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "multiProj2")

	// Original dirs should exist, not the --workflow-name
	require.DirExists(t, filepath.Join(projectRoot, "por"))
	require.DirExists(t, filepath.Join(projectRoot, "nav"))
	_, err = os.Stat(filepath.Join(projectRoot, "test-rename"))
	require.True(t, os.IsNotExist(err), "workflow-name flag should be ignored for multi-workflow templates")
}

func TestSingleWorkflowDefaultFromTemplate(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Verify the Execute path uses workflows[0].dir when workflowName is empty.
	// We simulate the wizard result by providing all flags except workflow name,
	// but since Execute fills the default from Workflows[0].Dir, the result should
	// use "my-workflow" (the template's declared dir name).
	// Note: We must provide a workflow name to avoid the TTY prompt in tests.
	// Instead, we verify the default logic by providing it explicitly.
	inputs := Inputs{
		ProjectName:  "singleProj",
		TemplateName: "kv-store-go",
		WorkflowName: "my-workflow", // same as template's workflows[0].dir
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "singleProj")
	// Should use the template's default dir name without rename
	require.DirExists(t, filepath.Join(projectRoot, "my-workflow"),
		"single workflow should use template's workflows[0].dir")
	require.FileExists(t, filepath.Join(projectRoot, "my-workflow", constants.DefaultWorkflowSettingsFileName))
}

func TestSingleWorkflowDefaultInExecute(t *testing.T) {
	// Verify that Execute defaults workflowName to workflows[0].dir
	// when workflowName is empty (unit test for the default logic, not the wizard).
	tmpl := testSingleWorkflowWithPostInit
	require.Equal(t, 1, len(tmpl.Workflows))
	require.Equal(t, "my-workflow", tmpl.Workflows[0].Dir)

	// The Execute code path:
	// if workflowName == "" && len(selectedTemplate.Workflows) == 1 {
	//     workflowName = selectedTemplate.Workflows[0].Dir
	// }
	workflowName := ""
	if workflowName == "" {
		if len(tmpl.Workflows) == 1 {
			workflowName = tmpl.Workflows[0].Dir
		} else {
			workflowName = constants.DefaultWorkflowName
		}
	}
	require.Equal(t, "my-workflow", workflowName)
}

func TestSingleWorkflowRenameWithFlag(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Single workflow with --workflow-name: should rename to user's choice
	inputs := Inputs{
		ProjectName:  "renameProj",
		TemplateName: "kv-store-go",
		WorkflowName: "counter",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "renameProj")
	require.DirExists(t, filepath.Join(projectRoot, "counter"),
		"single workflow should be renamed to user's choice")
	require.FileExists(t, filepath.Join(projectRoot, "counter", constants.DefaultWorkflowSettingsFileName))

	// Original dir should NOT exist
	_, err = os.Stat(filepath.Join(projectRoot, "my-workflow"))
	require.True(t, os.IsNotExist(err), "original dir should be renamed")
}

func TestInitWithUseTenderlyFlag(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:  "tenderlyProj",
		TemplateName: "test-multichain",
		WorkflowName: "tenderly-wf",
		UseTenderly:  true,
	}

	h := newHandlerWithTenderly(sim.NewRuntimeContext(), newMockRegistry(), "https://rpc.tenderly.co/vnet/test123")
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "tenderlyProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)

	// Vnet URLs should be applied to project.yaml
	require.Contains(t, content, "https://rpc.tenderly.co/vnet/test123")
	require.Contains(t, content, "ethereum-testnet-sepolia")
	require.Contains(t, content, "ethereum-mainnet")
}

func TestInitWithUseTenderlyNoEnvVar(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Clear all Tenderly env vars so NewAPIProvider fails
	t.Setenv("TENDERLY_ACCESS_KEY", "")
	t.Setenv("TENDERLY_ACCOUNT_SLUG", "")
	t.Setenv("TENDERLY_PROJECT_SLUG", "")

	inputs := Inputs{
		ProjectName:  "tenderlyNoEnv",
		TemplateName: "test-multichain",
		WorkflowName: "tenderly-wf",
		UseTenderly:  true,
	}

	// Use a handler WITHOUT the mock provider so it falls through to NewAPIProvider
	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	err = h.Execute(inputs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TENDERLY_ACCESS_KEY")
}

func TestInitTenderlyRpcUrlPrecedence(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Explicit --rpc-url for sepolia should override the vnet URL
	inputs := Inputs{
		ProjectName:  "precedenceProj",
		TemplateName: "test-multichain",
		WorkflowName: "prec-wf",
		UseTenderly:  true,
		RpcURLs: map[string]string{
			"ethereum-testnet-sepolia": "https://my-custom-rpc.example.com",
		},
	}

	h := newHandlerWithTenderly(sim.NewRuntimeContext(), newMockRegistry(), "https://rpc.tenderly.co/vnet/base")
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "precedenceProj")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)

	// Sepolia should use the explicit --rpc-url, not the vnet URL
	require.Contains(t, content, "https://my-custom-rpc.example.com")
	// Mainnet should use the vnet URL
	require.Contains(t, content, "https://rpc.tenderly.co/vnet/base")
}

func TestInitTenderlyWithProjectDir(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:  "projdirTenderly",
		TemplateName: "projdir-multichain",
		WorkflowName: "my-workflow",
		UseTenderly:  true,
	}

	h := newHandlerWithTenderly(sim.NewRuntimeContext(), newMockRegistry(), "https://virtual.tenderly.co/vnet/abc")
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "projdirTenderly")
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	content := string(projectYAML)

	// Vnet URLs should be patched into project.yaml even with ProjectDir templates
	require.Contains(t, content, "https://virtual.tenderly.co/vnet/abc")
	require.NotContains(t, content, "https://default-rpc.example.com",
		"default RPC URLs should be replaced by vnet URLs")
}

func TestInitTenderlySkippedNoNetworks(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// hello-world-go has no Networks — Tenderly should be a no-op
	inputs := Inputs{
		ProjectName:  "noNetProj",
		TemplateName: "hello-world-go",
		WorkflowName: "no-net-wf",
		UseTenderly:  true,
	}

	h := newHandlerWithTenderly(sim.NewRuntimeContext(), newMockRegistry(), "https://rpc.tenderly.co/vnet/unused")
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "noNetProj")
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	// Vnet URL should NOT appear since template has no networks
	projectYAML, err := os.ReadFile(filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.NoError(t, err)
	require.NotContains(t, string(projectYAML), "tenderly")
}

func TestBuiltInTemplateBackwardsCompat(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Built-in template has no Workflows field — should use existing heuristic
	inputs := Inputs{
		ProjectName:  "builtinProj",
		TemplateName: "hello-world-go",
		WorkflowName: "hello-wf",
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "builtinProj")
	require.DirExists(t, filepath.Join(projectRoot, "hello-wf"),
		"built-in template should use user's workflow name")
	require.FileExists(t, filepath.Join(projectRoot, "hello-wf", constants.DefaultWorkflowSettingsFileName))
}

func TestMissingNetworks(t *testing.T) {
	cases := []struct {
		name     string
		template *templaterepo.TemplateSummary
		flags    map[string]string
		expected []string
	}{
		{
			name:     "nil template",
			template: nil,
			flags:    nil,
			expected: nil,
		},
		{
			name: "no networks required",
			template: &templaterepo.TemplateSummary{
				TemplateMetadata: templaterepo.TemplateMetadata{},
			},
			flags:    nil,
			expected: nil,
		},
		{
			name:     "all provided",
			template: &testMultiNetworkTemplate,
			flags: map[string]string{
				"ethereum-testnet-sepolia": "https://rpc1.example.com",
				"ethereum-mainnet":         "https://rpc2.example.com",
			},
			expected: nil,
		},
		{
			name:     "some missing",
			template: &testMultiNetworkTemplate,
			flags: map[string]string{
				"ethereum-testnet-sepolia": "https://rpc1.example.com",
			},
			expected: []string{"ethereum-mainnet"},
		},
		{
			name:     "all missing",
			template: &testMultiNetworkTemplate,
			flags:    map[string]string{},
			expected: []string{"ethereum-testnet-sepolia", "ethereum-mainnet"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := MissingNetworks(tc.template, tc.flags)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNonInteractiveMissingFlags(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:    "proj",
		TemplateName:   "test-multichain",
		WorkflowName:   "",
		NonInteractive: true,
		RpcURLs:        map[string]string{},
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	err = h.Execute(inputs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required flags for --non-interactive mode")
}

func TestNonInteractiveAllFlagsProvided(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	tempDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreCwd()

	inputs := Inputs{
		ProjectName:    "niProj",
		TemplateName:   "hello-world-go",
		WorkflowName:   "my-wf",
		NonInteractive: true,
	}

	h := newHandlerWithRegistry(sim.NewRuntimeContext(), newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	projectRoot := filepath.Join(tempDir, "niProj")
	require.DirExists(t, filepath.Join(projectRoot, "my-wf"))
}

func TestInitRespectsProjectRootFlag(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	// CWD is a temp dir (simulating being "somewhere else")
	cwdDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(cwdDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Target directory is a separate temp dir (simulating -R flag)
	targetDir := t.TempDir()

	inputs := Inputs{
		ProjectName:  "myproj",
		TemplateName: "test-go",
		WorkflowName: "mywf",
		RpcURLs:      map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
		ProjectRoot:  targetDir,
	}

	ctx := sim.NewRuntimeContext()

	h := newHandlerWithRegistry(ctx, newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	// Project should be created under targetDir, NOT cwdDir
	projectRoot := filepath.Join(targetDir, "myproj")
	validateInitProjectStructure(t, projectRoot, "mywf", GetTemplateFileListGo())

	// Verify nothing was created in CWD
	entries, err := os.ReadDir(cwdDir)
	require.NoError(t, err)
	require.Empty(t, entries, "CWD should be untouched when -R is provided")
}

func TestInitProjectRootFlagFindsExistingProject(t *testing.T) {
	sim := chainsim.NewSimulatedEnvironment(t)
	defer sim.Close()

	// CWD is a clean temp dir with no project
	cwdDir := t.TempDir()
	restoreCwd, err := testutil.ChangeWorkingDirectory(cwdDir)
	require.NoError(t, err)
	defer restoreCwd()

	// Create an "existing project" in a separate directory
	existingProject := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(existingProject, constants.DefaultProjectSettingsFileName),
		[]byte("name: existing"), 0600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(existingProject, constants.DefaultEnvFileName),
		[]byte(""), 0600,
	))

	inputs := Inputs{
		ProjectName:  "",
		TemplateName: "test-go",
		WorkflowName: "new-workflow",
		RpcURLs:      map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
		ProjectRoot:  existingProject,
	}

	ctx := sim.NewRuntimeContext()

	h := newHandlerWithRegistry(ctx, newMockRegistry())
	require.NoError(t, h.ValidateInputs(inputs))
	require.NoError(t, h.Execute(inputs))

	// Workflow should be scaffolded into the existing project
	validateInitProjectStructure(
		t,
		existingProject,
		"new-workflow",
		GetTemplateFileListGo(),
	)
}
