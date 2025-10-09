package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testEVMConfig struct {
	TokenAddress          string `json:"tokenAddress"`
	ReserveManagerAddress string `json:"reserveManagerAddress"`
	BalanceReaderAddress  string `json:"balanceReaderAddress"`
	MessageEmitterAddress string `json:"messageEmitterAddress"`
	ChainName             string `json:"chainName"`
	GasLimit              uint64 `json:"gasLimit"`
}
type testWorkflowConfig struct {
	Schedule string          `json:"schedule"`
	URL      string          `json:"url"`
	EVMs     []testEVMConfig `json:"evms"`
}

// Spins up a local HTTP server that returns a PORResponse.
func startMockPORServer(t *testing.T) *httptest.Server {
	t.Helper()
	// Matches fields used by fetchPOR()
	type porResponse struct {
		AccountName string    `json:"accountName"`
		TotalTrust  float64   `json:"totalTrust"`
		TotalToken  float64   `json:"totalToken"`
		Ripcord     bool      `json:"ripcord"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := porResponse{
			AccountName: "mock-account",
			TotalTrust:  1.0,
			TotalToken:  123.456, // your workflow scales this to 1e18 later
			Ripcord:     false,
			UpdatedAt:   time.Now().UTC(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Overwrite the workflow's config.json URL to point at the mock server.
func patchWorkflowConfigURL(t *testing.T, projectRoot string, workflowName string, newURL string) {
	t.Helper()
	cfgPath := filepath.Join(projectRoot, workflowName, "config.json")
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err, "read config.json")

	var cfg testWorkflowConfig
	require.NoError(t, json.Unmarshal(raw, &cfg), "unmarshal config.json")

	cfg.URL = newURL

	out, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err, "marshal patched config")
	require.NoError(t, os.WriteFile(cfgPath, out, 0o644), "write patched config")
}

// Simulates a workflow
func RunSimulationHappyPath(t *testing.T, tc TestConfig, projectDir string) {
	t.Helper()

	t.Run("Simulate", func(t *testing.T) {
		srv := startMockPORServer(t)
		patchWorkflowConfigURL(t, projectDir, "chainreader_workflow", srv.URL)

		// Build CLI args
		args := []string{
			"workflow", "simulate",
			"chainreader_workflow",
			tc.GetCliEnvFlag(),
			tc.GetProjectRootFlag(),
			"--non-interactive",
			"--trigger-index=0",
		}

		cmd := exec.Command(CLIPath, args...)

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(
			t,
			cmd.Run(),
			"cre workflow simulation failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
			stdout.String(),
			stderr.String(),
		)

		out := StripANSI(stdout.String() + stderr.String())

		require.Contains(t, out, "Workflow compiledsadf", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Simulator Initialized", "expected workflow to initialize.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Getting native balances", "expected workflow to read from balance reader.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Workflow Simulation Result", "expected simulation success.\nCLI OUTPUT:\n%s", out)
	})
}
