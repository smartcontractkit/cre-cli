package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// TestE2EInit_ConvertToCustomBuild_TS: init (typescriptSimpleExample), bun install, simulate (capture),
// convert (copies script from node_modules), make build, simulate (require match), then patch .cre/compile-to-js
// for declare/inject and main.ts for BUILD_FLAG, make with FLAG=customFlag/differentFlag, simulate and assert.
func TestE2EInit_ConvertToCustomBuild_TS(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-convert-ts"
	workflowName := "tsWorkflow"
	templateID := "3" // typescriptSimpleExample
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	t.Setenv(settings.EthPrivateKeyEnvVar, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := NewGraphQLMockServerGetOrganization(t)
	defer gqlSrv.Close()

	// --- cre init with typescriptSimpleExample ---
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
	require.FileExists(t, filepath.Join(workflowDirectory, "main.ts"))

	// bun install so simulate can build and convert can find node_modules
	installCmd := exec.Command("bun", "install")
	installCmd.Dir = workflowDirectory
	installCmd.Stdout = &stdout
	installCmd.Stderr = &stderr
	require.NoError(t, installCmd.Run(), "bun install failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	// Before/after: simulate (capture), convert, make build, simulate (verify same key content)
	ConvertSimulateBeforeAfter(t, projectRoot, workflowDirectory, workflowName, "Hello world!")
	require.FileExists(t, filepath.Join(workflowDirectory, "Makefile"))
	require.DirExists(t, filepath.Join(workflowDirectory, "wasm"))
	require.FileExists(t, filepath.Join(workflowDirectory, ".cre", "compile-to-js.ts"))
	require.FileExists(t, filepath.Join(workflowDirectory, ".cre", "workflow-wrapper.ts"))

	// Patch .cre/compile-to-js.ts to inject BUILD_FLAG from env FLAG at build time (declare + inject)
	compileToJSPath := filepath.Join(workflowDirectory, ".cre", "compile-to-js.ts")
	compileToJSBytes, err := os.ReadFile(compileToJSPath)
	require.NoError(t, err)
	compileToJSSrc := string(compileToJSBytes)
	definePatch := "define: process.env.FLAG !== undefined ? { BUILD_FLAG: JSON.stringify(process.env.FLAG) } : {},"
	compileToJSSrc = strings.Replace(compileToJSSrc,
		"naming: path.basename(resolvedOutput),\n\t	})",
		"naming: path.basename(resolvedOutput),\n\t\t"+definePatch+"\n\t	})",
		1)
	require.NoError(t, os.WriteFile(compileToJSPath, []byte(compileToJSSrc), 0600))

	// Patch main.ts: declare const BUILD_FLAG and return message based on it
	mainPath := filepath.Join(workflowDirectory, "main.ts")
	mainBytes, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	mainStr := string(mainBytes)
	mainStr = "declare const BUILD_FLAG: string;\n" + mainStr
	mainStr = strings.Replace(mainStr, "return \"Hello world!\";",
		"return BUILD_FLAG === 'customFlag' ? \"Hello World (custom)\" : \"Hello World (default)\";", 1)
	require.NoError(t, os.WriteFile(mainPath, []byte(mainStr), 0600))

	// Patch Makefile to pass FLAG so the script sees process.env.FLAG
	makefilePath := filepath.Join(workflowDirectory, "Makefile")
	makefileBytes, err := os.ReadFile(makefilePath)
	require.NoError(t, err)
	makefileStr := string(makefileBytes)
	makefileStr = strings.Replace(makefileStr, "bun .cre/compile-to-js.ts", "FLAG=$(FLAG) bun .cre/compile-to-js.ts", 1)
	require.NoError(t, os.WriteFile(makefilePath, []byte(makefileStr), 0600))

	convertTSBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=customFlag", "Hello World (custom)")
	convertTSBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=differentFlag", "Hello World (default)")
}

func convertTSBuildWithFlagAndAssert(t *testing.T, projectRoot, workflowDir, workflowName, envVar, wantSubstr string) {
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
	require.NoError(t, cmd.Run(), "simulate failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), wantSubstr)
}
