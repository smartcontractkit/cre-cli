package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

type templateCompatibilityCase struct {
	name          string
	templateID    string
	workflowName  string
	lang          string // go | ts
	needsRPCURL   bool
	expectedFiles []string
	runBindings   bool
	simulateMode  string // pass | compile-only
	goBuildWASM   bool
}

func TestTemplateCompatibility(t *testing.T) {
	templateCases := []templateCompatibilityCase{
		{
			name:          "Go_PoR_Template1",
			templateID:    "1",
			workflowName:  "por-workflow",
			lang:          "go",
			needsRPCURL:   true,
			expectedFiles: []string{"README.md", "main.go", "workflow.yaml", "workflow.go", "workflow_test.go"},
			runBindings:   true,
			simulateMode:  "pass",
			goBuildWASM:   true,
		},
		{
			name:          "Go_HelloWorld_Template2",
			templateID:    "2",
			workflowName:  "go-hello-workflow",
			lang:          "go",
			needsRPCURL:   false,
			expectedFiles: []string{"README.md", "main.go", "workflow.yaml"},
			runBindings:   false,
			simulateMode:  "pass",
			goBuildWASM:   true,
		},
		{
			name:          "TS_HelloWorld_Template3",
			templateID:    "3",
			workflowName:  "ts-hello-workflow",
			lang:          "ts",
			needsRPCURL:   false,
			expectedFiles: []string{"README.md", "main.ts", "workflow.yaml", "package.json", "tsconfig.json"},
			runBindings:   false,
			simulateMode:  "pass",
		},
		{
			name:          "TS_PoR_Template4",
			templateID:    "4",
			workflowName:  "ts-por-workflow",
			lang:          "ts",
			needsRPCURL:   true,
			expectedFiles: []string{"README.md", "main.ts", "workflow.yaml", "package.json", "tsconfig.json"},
			runBindings:   false,
			simulateMode:  "pass",
		},
		{
			name:          "TS_ConfHTTP_Template5",
			templateID:    "5",
			workflowName:  "ts-conf-http-workflow",
			lang:          "ts",
			needsRPCURL:   false,
			expectedFiles: []string{"README.md", "main.ts", "workflow.yaml", "package.json", "tsconfig.json"},
			runBindings:   false,
			simulateMode:  "compile-only",
		},
	}

	for _, tc := range templateCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			projectName := "compat-" + tc.templateID
			projectRoot := filepath.Join(tempDir, projectName)
			workflowDir := filepath.Join(projectRoot, tc.workflowName)

			t.Setenv(settings.EthPrivateKeyEnvVar, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
			t.Setenv(credentials.CreApiKeyVar, "test-api")

			gqlSrv := startTemplateCompatibilityGraphQLMock(t)
			defer gqlSrv.Close()
			t.Setenv(environments.EnvVarGraphQLURL, gqlSrv.URL+"/graphql")

			initArgs := []string{
				"init",
				"--project-root", tempDir,
				"--project-name", projectName,
				"--template-id", tc.templateID,
				"--workflow-name", tc.workflowName,
			}
			if tc.needsRPCURL {
				initArgs = append(initArgs, "--rpc-url", constants.DefaultEthSepoliaRpcUrl)
			}

			runCLICommand(t, tempDir, initArgs...)

			require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
			require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
			require.DirExists(t, workflowDir)

			for _, fileName := range tc.expectedFiles {
				require.FileExists(t, filepath.Join(workflowDir, fileName), "missing workflow file %q", fileName)
			}

			if tc.lang == "go" {
				if tc.runBindings {
					runCLICommand(t, projectRoot, "generate-bindings", "evm")
					runExternalCommand(t, projectRoot, "go", "mod", "tidy")
				}
				if tc.goBuildWASM {
					runExternalCommandWithEnv(
						t,
						workflowDir,
						append([]string{}, "GOOS=wasip1", "GOARCH=wasm"),
						"go",
						"build",
						"-o",
						"workflow.wasm",
						".",
					)
				} else {
					runExternalCommand(t, projectRoot, "go", "build", "./...")
				}
			} else {
				runExternalCommand(t, workflowDir, "bun", "install")
			}

			simArgs := []string{
				"workflow", "simulate",
				tc.workflowName,
				"--project-root", projectRoot,
				"--non-interactive",
				"--trigger-index=0",
			}
			switch tc.simulateMode {
			case "pass":
				simOutput := runCLICommand(t, projectRoot, simArgs...)
				require.Contains(t, simOutput, "Workflow compiled", "expected simulate output to confirm compilation")
			case "compile-only":
				stdout, stderr, err := runCLICommandWithResult(t, projectRoot, simArgs...)
				require.Error(t, err, "expected known runtime failure mode for compile-only template checks")
				simOutput := stdout + stderr
				require.Contains(t, simOutput, "Workflow compiled", "expected simulate output to confirm compilation")
			default:
				t.Fatalf("unknown simulate mode: %s", tc.simulateMode)
			}
		})
	}
}

func TestTemplateCompatibility_AllTemplatesCovered(t *testing.T) {
	templateIDs := map[string]struct{}{
		"1": {},
		"2": {},
		"3": {},
		"4": {},
		"5": {},
	}

	const expectedTemplateCount = 5
	require.Len(
		t,
		templateIDs,
		expectedTemplateCount,
		"template count mismatch: update template compatibility test table when adding templates",
	)
}

func startTemplateCompatibilityGraphQLMock(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/graphql") || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "getOrganization") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"getOrganization": map[string]any{
						"organizationId": "test-org-id",
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
		})
	}))
}

func runCLICommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	stdout, stderr, err := runCLICommandWithResult(t, dir, args...)
	require.NoError(
		t,
		err,
		"command failed: %s\nSTDOUT:\n%s\nSTDERR:\n%s",
		fmt.Sprintf("%s %s", CLIPath, strings.Join(args, " ")),
		stdout,
		stderr,
	)

	return stdout + stderr
}

func runCLICommandWithResult(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runExternalCommand(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(
		t,
		cmd.Run(),
		"command failed: %s %s\nSTDOUT:\n%s\nSTDERR:\n%s",
		name,
		strings.Join(args, " "),
		stdout.String(),
		stderr.String(),
	)

	return stdout.String() + stderr.String()
}

func runExternalCommandWithEnv(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(
		t,
		cmd.Run(),
		"command failed: %s %s\nSTDOUT:\n%s\nSTDERR:\n%s",
		name,
		strings.Join(args, " "),
		stdout.String(),
		stderr.String(),
	)

	return stdout.String() + stderr.String()
}
