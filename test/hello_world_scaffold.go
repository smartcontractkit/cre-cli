package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
)

// scaffoldHelloWorldGoProject lays out the built-in hello-world-go template with
// project/workflow settings and Go module deps (without cre init).
func scaffoldHelloWorldGoProject(t *testing.T, projectRoot, workflowName string) {
	t.Helper()

	logger := testutil.NewTestLogger()
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	require.NoError(t, templaterepo.ScaffoldBuiltIn(logger, "hello-world-go", projectRoot, workflowName))

	projectYAML := `staging-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://sepolia.infura.io/v3
production-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://sepolia.infura.io/v3
`
	workflowYAML := fmt.Sprintf(`staging-settings:
  user-workflow:
    workflow-name: "%s-staging"
  workflow-artifacts:
    workflow-path: "./main.go"
    config-path: "./config.staging.json"
    secrets-path: "../secrets.yaml"
production-settings:
  user-workflow:
    workflow-name: "%s-production"
  workflow-artifacts:
    workflow-path: "./main.go"
    config-path: "./config.production.json"
    secrets-path: "../secrets.yaml"
`, workflowName, workflowName)

	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName),
		[]byte(projectYAML),
		0600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectRoot, workflowName, constants.DefaultWorkflowSettingsFileName),
		[]byte(workflowYAML),
		0600,
	))

	initializeHelloWorldGoModule(t, projectRoot)
}

func initializeHelloWorldGoModule(t *testing.T, projectRoot string) {
	t.Helper()

	moduleName := filepath.Base(projectRoot)
	cmd := exec.Command("go", "mod", "init", moduleName)
	cmd.Dir = projectRoot
	require.NoError(t, cmd.Run(), "go mod init failed")

	// cron@v1.3.0 does not resolve on the public module proxy (pulls cre-sdk-go@v1.3.0).
	// v1.0.0-beta.0 matches test/test_project/blank_workflow and works with SdkVersion.
	deps := []string{
		"github.com/smartcontractkit/cre-sdk-go@" + constants.SdkVersion,
		"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron@v1.0.0-beta.0",
	}
	for _, dep := range deps {
		getCmd := exec.Command("go", "get", dep)
		getCmd.Dir = projectRoot
		out, err := getCmd.CombinedOutput()
		require.NoError(t, err, "go get %s failed:\n%s", dep, out)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = projectRoot
	out, err := tidyCmd.CombinedOutput()
	require.NoError(t, err, "go mod tidy failed:\n%s", out)
}
