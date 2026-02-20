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
// convert, simulate (verify same). Verify conversion did not change main.ts. Then test-only: copy
// workflow-wrapper, write custom compile-to-js with define section in Bun.build, patch main.ts, Makefile.
// make with FLAG=customFlag/differentFlag, simulate and assert.
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

	// bun install so simulate can build
	installCmd := exec.Command("bun", "install")
	installCmd.Dir = workflowDirectory
	installCmd.Stdout = &stdout
	installCmd.Stderr = &stderr
	require.NoError(t, installCmd.Run(), "bun install failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	ConvertSimulateBeforeAfter(t, projectRoot, workflowDirectory, workflowName, "Hello world!")
	require.FileExists(t, filepath.Join(workflowDirectory, "Makefile"))
	require.DirExists(t, filepath.Join(workflowDirectory, "wasm"))

	// Verify conversion did not change main.ts
	mainPath := filepath.Join(workflowDirectory, "main.ts")
	mainBefore, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	require.Contains(t, string(mainBefore), `return "Hello world!";`, "convert must not modify workflow source")

	// Test-only: copy compile-to-js and workflow-wrapper from SDK, then patch to add define (so FLAG env drives the build).
	scriptsDir := filepath.Join(workflowDirectory, "scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0755))
	srcDir := filepath.Join(workflowDirectory, "node_modules", "@chainlink", "cre-sdk", "scripts", "src")
	for _, name := range []string{"compile-to-js.ts", "workflow-wrapper.ts"} {
		b, err := os.ReadFile(filepath.Join(srcDir, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, name), b, 0600))
	}
	compileToJSPath := filepath.Join(scriptsDir, "compile-to-js.ts")
	compileToJS, err := os.ReadFile(compileToJSPath)
	require.NoError(t, err)
	src := string(compileToJS)
	// Use local workflow-wrapper (script is under scripts/, not SDK entry)
	if !strings.Contains(src, "workflow-wrapper") {
		src = strings.Replace(src, `import { $ } from "bun";`, `import { $ } from "bun";
import { wrapWorkflowCode } from "./workflow-wrapper";`, 1)
	}
	// Argv slice(2) so "bun scripts/compile-to-js.ts main.ts wasm/workflow.js" passes both args
	// it's called differently than the SDK so we need to patch it
	src = strings.Replace(src, "process.argv.slice(3)", "process.argv.slice(2)", 1)

	defineBlock := "define: {\n\t\t\tBUILD_FLAG: JSON.stringify(process.env.FLAG ?? \"\"),\n\t\t},\n\t\t"
	anchor := "naming: path.basename(resolvedOutput),"
	if idx := strings.Index(src, anchor); idx >= 0 {
		src = src[:idx] + defineBlock + src[idx:]
	}
	require.Contains(t, src, "define:", "patch must add define section to Bun.build")
	if !strings.Contains(src, "main().catch") && !strings.Contains(src, "await main()") {
		src = src + "\nmain().catch((err: unknown) => { console.error(err); process.exit(1); });\n"
	}
	require.NoError(t, os.WriteFile(compileToJSPath, []byte(src), 0600))

	mainStr := string(mainBefore)
	mainStr = "declare const BUILD_FLAG: string;\n" + mainStr
	newReturn := `return BUILD_FLAG === "customFlag" ? "Hello World (custom)" : "Hello World (default)";`
	for _, oldReturn := range []string{`  return "Hello world!";`, `return "Hello world!";`} {
		if strings.Contains(mainStr, oldReturn) {
			mainStr = strings.Replace(mainStr, oldReturn, newReturn, 1)
			break
		}
	}
	require.Contains(t, mainStr, "Hello World (custom)", "main.ts return patch must apply")
	require.NoError(t, os.WriteFile(mainPath, []byte(mainStr), 0600))

	makefilePath := filepath.Join(workflowDirectory, "Makefile")
	makefileContent := `.PHONY: build

build:
	FLAG=$(FLAG) bun scripts/compile-to-js.ts main.ts wasm/workflow.js
	bunx cre-compile-workflow wasm/workflow.js wasm/workflow.wasm
`
	require.NoError(t, os.WriteFile(makefilePath, []byte(makefileContent), 0600))

	convertTSBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=customFlag", "Hello World (custom)")
	convertTSBuildWithFlagAndAssert(t, projectRoot, workflowDirectory, workflowName, "FLAG=differentFlag", "Hello World (default)")
}

func convertTSBuildWithFlagAndAssert(t *testing.T, projectRoot, workflowDir, workflowName, envVar, wantSubstr string) {
	t.Helper()
	convertRunMakeBuild(t, workflowDir, envVar)
	var stdout, stderr bytes.Buffer
	workflowDirAbs, err := filepath.Abs(workflowDir)
	require.NoError(t, err)
	cmd := exec.Command(CLIPath, "workflow", "simulate", workflowDirAbs,
		"--project-root", projectRoot,
		"--non-interactive", "--trigger-index=0",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Simulate runs CompileWorkflowToWasm which runs make build again; pass env so the rebuild uses the same FLAG
	if envVar != "" {
		cmd.Env = append(os.Environ(), envVar)
	}
	require.NoError(t, cmd.Run(), "simulate failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), wantSubstr)
}
