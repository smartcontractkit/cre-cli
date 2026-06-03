// Build hello-world workflow WASM and copy into web/wasm-simulate-demo/assets/.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		fatal(fmt.Errorf("run from cre-cli repo root (go.mod not found in %s)", repoRoot))
	}

	assetsDir := filepath.Join(repoRoot, "web", "wasm-simulate-demo", "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		fatal(err)
	}

	tmp := filepath.Join(os.TempDir(), "cre-wasm-demo-build")
	_ = os.RemoveAll(tmp)
	projectRoot := filepath.Join(tmp, "demo-project")
	workflowName := "helloWorkflow"
	workflowDir := filepath.Join(projectRoot, workflowName)

	logger := testutil.NewTestLogger()
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		fatal(err)
	}
	if err := templaterepo.ScaffoldBuiltIn(logger, "hello-world-go", projectRoot, workflowName); err != nil {
		fatal(err)
	}

	modInit := exec.Command("go", "mod", "init", "wasm-demo")
	modInit.Dir = projectRoot
	if out, err := modInit.CombinedOutput(); err != nil {
		fatal(fmt.Errorf("go mod init: %w\n%s", err, out))
	}

	deps := []string{
		"github.com/smartcontractkit/cre-sdk-go@v1.11.0",
		"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron@v1.0.0-beta.0",
	}
	for _, dep := range deps {
		get := exec.Command("go", "get", dep)
		get.Dir = projectRoot
		if out, err := get.CombinedOutput(); err != nil {
			fatal(fmt.Errorf("go get %s: %w\n%s", dep, err, out))
		}
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = projectRoot
	if out, err := tidy.CombinedOutput(); err != nil {
		fatal(fmt.Errorf("go mod tidy: %w\n%s", err, out))
	}

	mainGo := filepath.Join(workflowDir, "main.go")
	wasmBytes, err := cmdcommon.CompileWorkflowToWasm(context.Background(), mainGo, cmdcommon.WorkflowCompileOptions{
		StripSymbols: true,
	})
	if err != nil {
		fatal(err)
	}
	wasmPath := filepath.Join(assetsDir, "hello-world.wasm")
	if err := os.WriteFile(wasmPath, wasmBytes, 0644); err != nil {
		fatal(err)
	}

	subscribeReq := &sdk.ExecuteRequest{
		Config:          []byte("{}"),
		MaxResponseSize: 2048,
		Request:         &sdk.ExecuteRequest_Subscribe{Subscribe: &emptypb.Empty{}},
	}
	subscribeBytes, err := proto.Marshal(subscribeReq)
	if err != nil {
		fatal(err)
	}

	manifest := map[string]any{
		"workflowWasm": "hello-world.wasm",
		"workflowUrl":  "./assets/hello-world.wasm",
		"manifestUrl":  "./assets/manifest.json",
		"wasmBytes":    len(wasmBytes),
		"subscribeB64": base64.StdEncoding.EncodeToString(subscribeBytes),
		"cronTypeUrl":  "type.googleapis.com/capabilities.scheduler.cron.v1.Payload",
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "manifest.json"), manifestJSON, 0644); err != nil {
		fatal(err)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", wasmPath, len(wasmBytes))
	fmt.Printf("Wrote %s\n", filepath.Join(assetsDir, "manifest.json"))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "build_assets: %v\n", err)
	os.Exit(1)
}
