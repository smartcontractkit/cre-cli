package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestE2EInit_WasmBlankTemplate(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-init-wasm-test"
	workflowName := "wasmWorkflow"
	templateID := "6"
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	ethKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	t.Setenv(settings.EthPrivateKeyEnvVar, ethKey)

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := NewGraphQLMockServerGetOrganization(t)
	defer gqlSrv.Close()

	// --- cre init with WASM template ---
	initArgs := []string{
		"init",
		"--project-root", tempDir,
		"--project-name", projectName,
		"--template-id", templateID,
		"--workflow-name", workflowName,
	}
	var stdout, stderr bytes.Buffer
	initCmd := exec.Command(CLIPath, initArgs...)
	initCmd.Dir = tempDir
	initCmd.Stdout = &stdout
	initCmd.Stderr = &stderr

	require.NoError(
		t,
		initCmd.Run(),
		"cre init failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.DirExists(t, workflowDirectory)

	expectedFiles := []string{"README.md", "Makefile", "workflow.yaml", "config.staging.json", "config.production.json", "secrets.yaml"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	// Create wasm directory
	wasmDir := filepath.Join(workflowDirectory, "wasm")
	require.NoError(t, os.MkdirAll(wasmDir, 0755))

	// Create a simple Go workflow file similar to blankTemplate but with custom build tag
	mainGoContent := `//go:build wasip1 && customwasm

package main

import (
	"fmt"
	"log/slog"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

type ExecutionResult struct {
	Result string
}

type Config struct{}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	cronTrigger := cron.Trigger(&cron.Config{Schedule: "*/30 * * * * *"})
	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, onCronTrigger),
	}, nil
}

func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*ExecutionResult, error) {
	logger := runtime.Logger()
	scheduledTime := trigger.ScheduledExecutionTime.AsTime()
	logger.Info("Cron trigger fired", "scheduledTime", scheduledTime)
	return &ExecutionResult{Result: fmt.Sprintf("Fired at %s", scheduledTime)}, nil
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
`
	mainGoPath := filepath.Join(workflowDirectory, "main.go")
	require.NoError(t, os.WriteFile(mainGoPath, []byte(mainGoContent), 0600))

	// Create go.mod file - will be updated by go mod tidy
	goModContent := `module wasm-workflow

go 1.25.3

require (
	github.com/smartcontractkit/cre-sdk-go v1.1.3
	github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron v1.0.0-beta.0
)
`
	goModPath := filepath.Join(workflowDirectory, "go.mod")
	require.NoError(t, os.WriteFile(goModPath, []byte(goModContent), 0600))

	// Update Makefile to include build command with custom build tag
	makefilePath := filepath.Join(workflowDirectory, "Makefile")
	makefileContent := `.PHONY: build

export GOOS := wasip1
export GOARCH := wasm

build:
	go build -tags customwasm -o wasm/workflow.wasm .
`
	require.NoError(t, os.WriteFile(makefilePath, []byte(makefileContent), 0600))

	// Run go mod tidy to resolve dependencies
	stdout.Reset()
	stderr.Reset()
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workflowDirectory
	tidyCmd.Stdout = &stdout
	tidyCmd.Stderr = &stderr

	require.NoError(
		t,
		tidyCmd.Run(),
		"go mod tidy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// Build the workflow using make build
	stdout.Reset()
	stderr.Reset()
	buildCmd := exec.Command("make", "build")
	buildCmd.Dir = workflowDirectory
	buildCmd.Stdout = &stdout
	buildCmd.Stderr = &stderr

	require.NoError(
		t,
		buildCmd.Run(),
		"make build failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// Verify WASM file was created
	wasmFilePath := filepath.Join(wasmDir, "workflow.wasm")
	require.FileExists(t, wasmFilePath, "WASM file should be created by make build")

	// --- cre workflow simulate wasmWorkflow ---
	stdout.Reset()
	stderr.Reset()
	simulateArgs := []string{
		"workflow", "simulate",
		workflowName,
		"--project-root", projectRoot,
		"--non-interactive",
		"--trigger-index=0",
	}
	simulateCmd := exec.Command(CLIPath, simulateArgs...)
	simulateCmd.Dir = projectRoot
	simulateCmd.Stdout = &stdout
	simulateCmd.Stderr = &stderr

	require.NoError(
		t,
		simulateCmd.Run(),
		"cre workflow simulate failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// --- cre workflow compile wasmWorkflow ---
	stdout.Reset()
	stderr.Reset()
	compileArgs := []string{
		"workflow", "compile",
		filepath.Join(workflowDirectory, "workflow.yaml"),
		"--project-root", projectRoot,
	}
	compileCmd := exec.Command(CLIPath, compileArgs...)
	compileCmd.Dir = projectRoot
	compileCmd.Stdout = &stdout
	compileCmd.Stderr = &stderr

	require.NoError(
		t,
		compileCmd.Run(),
		"cre workflow compile failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// Verify compiled output exists
	outputPath := filepath.Join(workflowDirectory, "binary.wasm.br.b64")
	require.FileExists(t, outputPath, "compiled output should exist")
}
