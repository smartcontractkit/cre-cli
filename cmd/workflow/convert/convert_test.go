package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func TestConvert_AlreadyWasm_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "foo-staging"
  workflow-artifacts:
    workflow-path: "./wasm/workflow.wasm"
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "foo-production"
  workflow-artifacts:
    workflow-path: "./wasm/workflow.wasm"
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))

	h := newHandler(nil)
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already a custom build")
}

func TestConvert_Force_UpdatesYAMLAndCreatesMakefile(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	mainGo := filepath.Join(dir, "main.go")
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "wf-staging"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "wf-production"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n"), 0600))

	h := newHandler(nil)
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: true})
	require.NoError(t, err)

	data, err := os.ReadFile(workflowYAML)
	require.NoError(t, err)
	require.Contains(t, string(data), wasmWorkflowPath)

	require.DirExists(t, filepath.Join(dir, "wasm"))
	makefile := filepath.Join(dir, "Makefile")
	require.FileExists(t, makefile)
	content, _ := os.ReadFile(makefile)
	require.Contains(t, string(content), "build")
	require.Contains(t, string(content), "wasm/workflow.wasm")
}

func TestConvert_PromptNo_Cancels(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	mainGo := filepath.Join(dir, "main.go")
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "wf-staging"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "wf-production"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n"), 0600))

	h := newHandler(nil)
	h.confirmFn = func(_ string, _ ...ui.ConfirmOption) (bool, error) { return false, nil }
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: false})
	require.NoError(t, err)

	data, err := os.ReadFile(workflowYAML)
	require.NoError(t, err)
	require.Contains(t, string(data), "workflow-path: \".\"")
	require.NotContains(t, string(data), wasmWorkflowPath)
	require.NoFileExists(t, filepath.Join(dir, "Makefile"))
}

func TestConvert_PromptYes_Proceeds(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	mainGo := filepath.Join(dir, "main.go")
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "wf-staging"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "wf-production"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n"), 0600))

	h := newHandler(nil)
	h.confirmFn = func(_ string, _ ...ui.ConfirmOption) (bool, error) { return true, nil }
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: false})
	require.NoError(t, err)

	data, err := os.ReadFile(workflowYAML)
	require.NoError(t, err)
	require.Contains(t, string(data), wasmWorkflowPath)
	require.FileExists(t, filepath.Join(dir, "Makefile"))
	require.DirExists(t, filepath.Join(dir, "wasm"))
}

func TestConvert_PromptEmpty_DefaultsYes_Proceeds(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	mainGo := filepath.Join(dir, "main.go")
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "wf-staging"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "wf-production"
  workflow-artifacts:
    workflow-path: "."
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n"), 0600))

	h := newHandler(nil)
	h.confirmFn = func(_ string, _ ...ui.ConfirmOption) (bool, error) { return true, nil }
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: false})
	require.NoError(t, err)

	data, err := os.ReadFile(workflowYAML)
	require.NoError(t, err)
	require.Contains(t, string(data), wasmWorkflowPath)
	require.FileExists(t, filepath.Join(dir, "Makefile"))
}

func TestConvert_TS_InstallsDepsIfNoNodeModules(t *testing.T) {
	dir := t.TempDir()
	workflowYAML := filepath.Join(dir, constants.DefaultWorkflowSettingsFileName)
	mainTS := filepath.Join(dir, "main.ts")
	packageJSON := filepath.Join(dir, "package.json")
	yamlContent := `staging-settings:
  user-workflow:
    workflow-name: "wf-staging"
  workflow-artifacts:
    workflow-path: "main.ts"
    config-path: "./config.staging.json"
production-settings:
  user-workflow:
    workflow-name: "wf-production"
  workflow-artifacts:
    workflow-path: "main.ts"
    config-path: "./config.production.json"
`
	require.NoError(t, os.WriteFile(workflowYAML, []byte(yamlContent), 0600))
	require.NoError(t, os.WriteFile(mainTS, []byte("export default function run() { return Promise.resolve({ result: \"ok\" }); }\n"), 0600))
	require.NoError(t, os.WriteFile(packageJSON, []byte(`{"name":"test","private":true,"dependencies":{"@chainlink/cre-sdk":"^1.0.3"}}`), 0600))

	h := newHandler(nil)
	err := h.Execute(Inputs{WorkflowFolder: dir, Force: true})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "Makefile"))
	makefile, _ := os.ReadFile(filepath.Join(dir, "Makefile"))
	require.Contains(t, string(makefile), "bun cre-compile", "Makefile should match CLI build")
	require.Contains(t, string(makefile), "main.ts", "Makefile should build main.ts")
	require.Contains(t, string(makefile), "wasm/workflow.wasm", "Makefile should output to wasm/workflow.wasm")

	// CLI must not change the workflow; main.ts unchanged
	mainTSContent, _ := os.ReadFile(mainTS)
	require.Contains(t, string(mainTSContent), "export default function run()", "convert must not modify workflow source")
}
