package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const simulationResultMarker = "Workflow Simulation Result"

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape codes from CLI output.
func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

type helloWorldExecutionResult struct {
	Result string `json:"Result"`
}

// extractSimulationResultJSON returns the JSON object printed after the simulation result marker.
func extractSimulationResultJSON(out string) (string, error) {
	idx := strings.Index(out, simulationResultMarker)
	if idx < 0 {
		return "", fmt.Errorf("%q not found in output", simulationResultMarker)
	}

	rest := out[idx+len(simulationResultMarker):]
	start := strings.Index(rest, "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON object after %q", simulationResultMarker)
	}

	dec := json.NewDecoder(strings.NewReader(rest[start:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return "", fmt.Errorf("decode simulation result JSON: %w", err)
	}
	return string(raw), nil
}

func parseHelloWorldSimulationResult(t *testing.T, out string) helloWorldExecutionResult {
	t.Helper()
	clean := stripANSI(out)
	jsonStr, err := extractSimulationResultJSON(clean)
	require.NoError(t, err, "output:\n%s", clean)

	var result helloWorldExecutionResult
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &result), "json: %s", jsonStr)
	return result
}

func assertHelloWorldSimulationResult(t *testing.T, out string) helloWorldExecutionResult {
	t.Helper()
	clean := stripANSI(out)
	require.Contains(t, clean, simulationResultMarker, "output:\n%s", clean)

	result := parseHelloWorldSimulationResult(t, out)
	require.Contains(t, result.Result, "Fired at", "Result field should contain cron timestamp prefix")
	return result
}

// runCLI runs the cre CLI from dir and returns combined stdout+stderr (for output parsing).
func runCLI(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"cre %s failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		strings.Join(args, " "),
		stdout.String(),
		stderr.String())
	return stdout.String() + stderr.String()
}
