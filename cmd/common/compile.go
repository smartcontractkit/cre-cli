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

var defaultWasmOutput = filepath.Join("wasm", "workflow.wasm")

// getBuildCmd returns a single step that builds the workflow and returns the WASM bytes.
func getBuildCmd(workflowRootFolder, mainFile, language string) (func() ([]byte, error), error) {
	tmpPath := filepath.Join(workflowRootFolder, ".cre_build_tmp.wasm")
	switch language {
	case constants.WorkflowLanguageTypeScript:
		cmd := exec.Command("bun", "cre-compile", mainFile, tmpPath)
		cmd.Dir = workflowRootFolder
		return func() ([]byte, error) {
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("%w\nbuild output:\n%s", err, strings.TrimSpace(string(out)))
			}
			b, err := os.ReadFile(tmpPath)
			_ = os.Remove(tmpPath)
			return b, err
		}, nil
	case constants.WorkflowLanguageGolang:
		// Build the package (.) so all .go files (main.go, workflow.go, etc.) are compiled together
		cmd := exec.Command(
			"go", "build",
			"-o", tmpPath,
			"-trimpath",
			"-ldflags=-buildid= -w -s",
			".",
		)
		cmd.Dir = workflowRootFolder
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
		return func() ([]byte, error) {
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("%w\nbuild output:\n%s", err, strings.TrimSpace(string(out)))
			}
			b, err := os.ReadFile(tmpPath)
			_ = os.Remove(tmpPath)
			return b, err
		}, nil
	case constants.WorkflowLanguageWasm:
		makeRoot, err := findMakefileRoot(workflowRootFolder)
		if err != nil {
			return nil, err
		}
		makeCmd := exec.Command("make", "build")
		makeCmd.Dir = makeRoot
		builtPath := filepath.Join(makeRoot, defaultWasmOutput)
		return func() ([]byte, error) {
			out, err := makeCmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("%w\nbuild output:\n%s", err, strings.TrimSpace(string(out)))
			}
			return os.ReadFile(builtPath)
		}, nil
	default:
		// Build the package (.) so all .go files are compiled together
		cmd := exec.Command(
			"go", "build",
			"-o", tmpPath,
			"-trimpath",
			"-ldflags=-buildid= -w -s",
			".",
		)
		cmd.Dir = workflowRootFolder
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
		return func() ([]byte, error) {
			out, err := cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("%w\nbuild output:\n%s", err, strings.TrimSpace(string(out)))
			}
			b, err := os.ReadFile(tmpPath)
			_ = os.Remove(tmpPath)
			return b, err
		}, nil
	}
}

// CompileWorkflowToWasm compiles the workflow at workflowPath and returns the WASM binary.
// It runs the sequence of commands from getBuildCmds (make build + copy for WASM, or single build for Go/TS), then reads the temp WASM file.
func CompileWorkflowToWasm(workflowPath string) ([]byte, error) {
	workflowRootFolder, workflowMainFile, err := WorkflowPathRootAndMain(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("workflow path: %w", err)
	}
	workflowAbsFile := filepath.Join(workflowRootFolder, workflowMainFile)
	language := GetWorkflowLanguage(workflowMainFile)

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

	buildStep, err := getBuildCmd(workflowRootFolder, workflowMainFile, language)
	if err != nil {
		return nil, err
	}
	wasm, err := buildStep()
	if err != nil {
		return nil, fmt.Errorf("failed to compile workflow: %w", err)
	}
	return wasm, nil
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
