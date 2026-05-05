package common

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const makefileName = "Makefile"

var defaultWasmOutput = filepath.Join("wasm", "workflow.wasm")

const (
	// SkipTypeChecksFlag is passed through to cre-compile for TypeScript workflows (matches @chainlink/cre-sdk).
	SkipTypeChecksFlag = "--skip-type-checks"
	// SkipTypeChecksCLIFlag is the Cobra/Viper flag name (no leading dashes).
	SkipTypeChecksCLIFlag = "skip-type-checks"
)

// WorkflowCompileOptions configures workflow compilation for CompileWorkflowToWasm.
type WorkflowCompileOptions struct {
	// StripSymbols, when true, strips debug symbols from Go WASM builds (smaller binary for deploy).
	StripSymbols bool
	// SkipTypeChecks, when true, passes SkipTypeChecksFlag to cre-compile for TypeScript workflows.
	SkipTypeChecks bool
}

// getBuildCmd returns a single step that builds the workflow and returns the WASM bytes.
func getBuildCmd(workflowRootFolder, mainFile, language string, opts WorkflowCompileOptions) (func() ([]byte, error), error) {
	tmpPath := filepath.Join(workflowRootFolder, ".cre_build_tmp.wasm")
	switch language {
	case constants.WorkflowLanguageTypeScript:
		args := []string{"cre-compile", mainFile, tmpPath}
		if opts.SkipTypeChecks {
			args = append(args, SkipTypeChecksFlag)
		}
		cmd := exec.Command("bun", args...)
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
		ldflags := "-buildid="
		if opts.StripSymbols {
			ldflags = "-buildid= -w -s"
		}
		cmd := exec.Command(
			"go", "build",
			"-o", tmpPath,
			"-trimpath",
			"-buildvcs=false",
			"-mod=readonly",
			"-ldflags="+ldflags,
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
		ldflags := "-buildid="
		if opts.StripSymbols {
			ldflags = "-buildid= -w -s"
		}
		cmd := exec.Command(
			"go", "build",
			"-o", tmpPath,
			"-trimpath",
			"-buildvcs=false",
			"-mod=readonly",
			"-ldflags="+ldflags,
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
// opts.StripSymbols: for Go builds, true strips debug symbols (deploy); false keeps them (simulate).
// opts.SkipTypeChecks: for TypeScript, passes SkipTypeChecksFlag to cre-compile.
// For custom Makefile WASM builds, StripSymbols and SkipTypeChecks have no effect.
func CompileWorkflowToWasm(workflowPath string, opts WorkflowCompileOptions) ([]byte, error) {
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
		warnGOTOOLCHAIN()
	case constants.WorkflowLanguageWasm:
		if err := EnsureTool("make"); err != nil {
			return nil, errors.New("make is required for WASM workflows but was not found in PATH")
		}
	default:
		return nil, fmt.Errorf("unsupported workflow language for file %s", workflowMainFile)
	}

	buildStep, err := getBuildCmd(workflowRootFolder, workflowMainFile, language, opts)
	if err != nil {
		return nil, err
	}
	wasm, err := buildStep()
	if err != nil {
		return nil, fmt.Errorf("failed to compile workflow: %w", err)
	}
	return wasm, nil
}

func warnGOTOOLCHAIN() {
	tc := os.Getenv("GOTOOLCHAIN")
	if tc == "" {
		ui.Warning("GOTOOLCHAIN is not set; the build may not be reproducible across environments. Set it in your .env.public file (e.g. GOTOOLCHAIN=go1.25.3).")
		return
	}

	envFile := settings.LoadedPublicEnvFilePath()
	if envFile == "" {
		ui.Warning(fmt.Sprintf("GOTOOLCHAIN=%s is set, but no .env.public file was loaded. The build will not be reproducible for others without the same environment variable.", tc))
		return
	}

	envVars := settings.LoadedPublicEnvVars()
	if _, ok := envVars["GOTOOLCHAIN"]; !ok {
		ui.Warning(fmt.Sprintf("GOTOOLCHAIN=%s is set, but is not in %s. The build will not be reproducible for others without the same environment variable.", tc, envFile))
	}
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
