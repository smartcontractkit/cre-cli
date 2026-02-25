package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// TestE2EInit_ConvertToCustomBuild_Go: init (blank Go), simulate (capture), convert, make build, simulate (require match),
// then add FlagProof/constA/constB/Makefile FLAG, make with FLAG=customFlag/differentFlag, simulate and assert.
func TestE2EInit_ConvertToCustomBuild_Go(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-convert-go"
	workflowName := "goWorkflow"
	templateID := "2" // blank Go template
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	t.Setenv(settings.EthPrivateKeyEnvVar, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := NewGraphQLMockServerGetOrganization(t)
	defer gqlSrv.Close()

	// --- cre init with blank Go template ---
	var stdout, stderr bytes.Buffer
	initCmd := exec.Command(CLIPath, "init",
		"--project-root", tempDir,
		"--project-name", projectName,
		"--template-id", templateID,
		"--workflow-name", workflowName,
	)
	initCmd.Dir = tempDir
	initCmd.Stdout = &stdout
	initCmd.Stderr = &stderr
	require.NoError(t, initCmd.Run(), "cre init failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.DirExists(t, workflowDirectory)
	require.FileExists(t, filepath.Join(workflowDirectory, "main.go"))

	// go mod tidy so simulate can build
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workflowDirectory
	tidyCmd.Stdout = &stdout
	tidyCmd.Stderr = &stderr
	require.NoError(t, tidyCmd.Run(), "go mod tidy failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	// Before/after: simulate (capture), convert, make build, simulate (verify same key content)
	ConvertSimulateBeforeAfter(t, projectRoot, workflowDirectory, workflowName, "Fired at")
	require.FileExists(t, filepath.Join(workflowDirectory, "Makefile"))
	require.DirExists(t, filepath.Join(workflowDirectory, "wasm"))

	// Now make test-specific changes: FlagProof, constA/constB, Makefile FLAG
	mainPath := filepath.Join(workflowDirectory, "workflow.go")
	mainBytes, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	mainStr := string(mainBytes)
	var nl = "\n"
	if runtime.GOOS == "windows" {
		nl = "\r\n"
	}

	mainStr = strings.Replace(mainStr, "type ExecutionResult struct {"+nl+"\tResult string"+nl+"}", "type ExecutionResult struct {"+nl+"\tResult    string"+nl+"\tFlagProof string"+nl+"}", 1)
	mainStr = strings.Replace(mainStr, "\t// Your logic here..."+nl+nl+"\treturn &ExecutionResult{Result: fmt.Sprintf(\"Fired at %s\", scheduledTime)}, nil",
		"\treturn &ExecutionResult{Result: fmt.Sprintf(\"Fired at %s\", scheduledTime), FlagProof: FlagProof}, nil", 1)

	require.NoError(t, os.WriteFile(mainPath, []byte(mainStr), 0600))

	constA := `//go:build customFlag

package main

const FlagProof = "set"
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowDirectory, "constA.go"), []byte(constA), 0600))

	constB := `//go:build !customFlag

package main

const FlagProof = "unset"
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowDirectory, "constB.go"), []byte(constB), 0600))

	makefilePath := filepath.Join(workflowDirectory, "Makefile")
	makefile, err := os.ReadFile(makefilePath)
	require.NoError(t, err)
	makefileStr := strings.Replace(string(makefile), "go build -o", "go build -tags $(FLAG) -o", 1)
	require.NoError(t, os.WriteFile(makefilePath, []byte(makefileStr), 0600))

	convertGoBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=customFlag", "set", "FlagProof")
	convertGoBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=differentFlag", "unset", "FlagProof")
}

func convertGoBuildWithFlagAndAssert(t *testing.T, projectRoot, workflowDir, workflowName, envVar, wantSubstr, wantSubstr2 string) {
	t.Helper()
	convertRunMakeBuild(t, workflowDir, envVar)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "workflow", "simulate", workflowName,
		"--project-root", projectRoot,
		"--non-interactive", "--trigger-index=0",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if envVar != "" {
		cmd.Env = append(os.Environ(), envVar)
	}
	require.NoError(t, cmd.Run(), "simulate failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), wantSubstr)
	require.Contains(t, stdout.String(), wantSubstr2)
}
