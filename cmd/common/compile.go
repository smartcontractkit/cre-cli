package common

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

const makefileName = "Makefile"

// CompileWorkflowToWasm compiles the workflow at workflowPath (value from workflow.yaml, e.g. ".",
// "main.go", "./wasm/workflow.wasm") and returns the WASM binary. For Go/TypeScript it runs
// go build or bun cre-compile; for WASM (path to .wasm file) it runs make build in the directory
// that contains the Makefile, then reads the WASM from the path specified in settings.
func CompileWorkflowToWasm(workflowPath string) ([]byte, error) {
	workflowRootFolder, workflowMainFile, err := WorkflowPathRootAndMain(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("workflow path: %w", err)
	}
	workflowAbsFile := filepath.Join(workflowRootFolder, workflowMainFile)
	language := GetWorkflowLanguage(workflowMainFile)

	// For Go/TypeScript the source file must exist; for WASM the .wasm file is produced by make.
	if language != constants.WorkflowLanguageWasm {
		if _, err := os.Stat(workflowAbsFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow file not found: %s", workflowAbsFile)
		}
	}

	switch language {
	case constants.WorkflowLanguageTypeScript:
		if err := EnsureTool("bun"); err != nil {
			return nil, errors.New("bun is required for TypeScript workflows but was not found in PATH; install from https://bun.com/docs/installation")
		}
	case constants.WorkflowLanguageGolang:
		if err := EnsureTool("go"); err != nil {
			return nil, errors.New("go toolchain is required for Go workflows but was not found in PATH; install from https://go.dev/dl")
		}
	case constants.WorkflowLanguageWasm:
		if err := EnsureTool("make"); err != nil {
			return nil, errors.New("make is required for WASM workflows but was not found in PATH")
		}
	default:
		return nil, fmt.Errorf("unsupported workflow language for file %s", workflowMainFile)
	}

	if language == constants.WorkflowLanguageWasm {
		// workflow-path points to the .wasm file; run make in the directory that contains the Makefile.
		makeRoot, err := findMakefileRoot(workflowRootFolder)
		if err != nil {
			return nil, err
		}
		buildCmd := exec.Command("make", "build")
		buildCmd.Dir = makeRoot
		buildOutput, err := buildCmd.CombinedOutput()
		if err != nil {
			out := strings.TrimSpace(string(buildOutput))
			return nil, fmt.Errorf("failed to build workflow: %w\nbuild output:\n%s", err, out)
		}
		return os.ReadFile(workflowAbsFile)
	}

	// Go or TypeScript: build to a temp file, read bytes, remove temp file.
	tmpWasm := filepath.Join(workflowRootFolder, ".cre_build_tmp.wasm")
	defer os.Remove(tmpWasm) // best-effort cleanup
	buildCmd := GetBuildCmd(workflowMainFile, tmpWasm, workflowRootFolder)
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		out := strings.TrimSpace(string(buildOutput))
		return nil, fmt.Errorf("failed to compile workflow: %w\nbuild output:\n%s", err, out)
	}
	_ = buildOutput
	return os.ReadFile(tmpWasm)
}

// findMakefileRoot walks up from dir and returns the first directory that contains a Makefile.
func findMakefileRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, makefileName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", errors.New("no Makefile found in directory or any parent (required for WASM workflow build)")
		}
		abs = parent
	}
}
