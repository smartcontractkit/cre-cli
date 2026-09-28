package test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/telemetry"
)

// commandsSkippedFromTelemetryE2E excepts login-exempt commands that reach a
// real external service or start an interactive flow. Everything else returned
// by cmd.LoginExemptCommandPaths() is exercised by default.
var commandsSkippedFromTelemetryE2E = map[string]string{
	"cre login":            "starts an interactive OAuth flow (opens a browser, waits for a callback)",
	"cre update":           "hits api.github.com for the latest release",
	"cre templates list":   "fetches the template list from GitHub",
	"cre templates add":    "fetches template contents from GitHub",
	"cre templates remove": "shares the same GitHub-backed template repo client as add/list",
}

// TestTelemetryForLoginExemptCommands verifies that commands excluded from
// credential loading (isLoadCredentials) still attach best-effort credentials
// so their telemetry reaches the server, instead of being silently dropped.
// It walks cmd.LoginExemptCommands directly, so a command added to that
// list later is covered automatically, with no test change needed.
func TestTelemetryForLoginExemptCommands(t *testing.T) {
	// help and completion are cobra built-ins that cobra only wires onto the
	// tree lazily inside Execute(); force that now so Find below can see them.
	cmd.RootCmd.InitDefaultHelpCmd()
	cmd.RootCmd.InitDefaultCompletionCmd()

	for _, path := range cmd.LoginExemptCommands {
		t.Run(path, func(t *testing.T) {
			if reason, skip := commandsSkippedFromTelemetryE2E[path]; skip {
				t.Skipf("skipped: %s", reason)
			}

			segments := strings.Fields(path)[1:] // drop the leading "cre"
			found, _, err := cmd.RootCmd.Find(segments)
			require.NoError(t, err, "could not resolve %q in the real command tree", path)

			if !found.Runnable() {
				t.Skipf("%q has no Run/RunE (a pure help group); cobra shows help before "+
					"PersistentPreRunE ever runs, so it can never emit telemetry regardless of credentials", path)
			}

			srv, rec := NewGraphQLMockServerRecordingTelemetry(t)
			defer srv.Close()

			t.Setenv(credentials.CreApiKeyVar, "test-api-key")

			args := append(append([]string{}, segments...), probeArgs(t, found)...)
			cliCmd := exec.Command(CLIPath, args...) // #nosec G204 -- args are derived from the real command tree above, not external input
			_ = cliCmd.Run()                         // exit code is irrelevant: most of these fail fast without a real project, but must still emit telemetry

			wantCount := 1
			if telemetry.ShouldExcludeCommand(found) {
				wantCount = 0 // e.g. version/help/completion: excluded by emitter.go independent of credentials
			}

			events := waitForTelemetryEventCount(t, rec, wantCount)
			require.Len(t, events, wantCount, "telemetry event count for %v", args)
			if wantCount == 0 {
				return
			}

			want := telemetry.CollectCommandInfo(found, nil)
			require.Equal(t, want.Action, events[0].Command.Action, "action for %v", args)
			require.Equal(t, want.Subcommand, events[0].Command.Subcommand, "subcommand for %v", args)
		})
	}
}

// TestTelemetrySkippedWithoutCredentials is the negative control: without any
// credentials on disk or in the environment, telemetry must stay silent rather
// than send an unauthenticated event, confirming credentials are what gate delivery.
func TestTelemetrySkippedWithoutCredentials(t *testing.T) {
	srv, rec := NewGraphQLMockServerRecordingTelemetry(t)
	defer srv.Close()

	t.Setenv(credentials.CreApiKeyVar, "")

	cliCmd := exec.Command(CLIPath, "generate-bindings", "evm")
	_ = cliCmd.Run()

	time.Sleep(300 * time.Millisecond) // grace period; nothing should arrive, late or not
	require.Empty(t, rec.Events(), "expected no telemetry events without credentials")
}

// probeArgs returns the smallest slice of placeholder positional args that
// satisfies found's Args validator, so commands requiring e.g. exactly one
// argument (like "workflow build <path>") can still be invoked generically.
func probeArgs(t *testing.T, found *cobra.Command) []string {
	t.Helper()
	if found.Args == nil {
		return nil
	}
	for n := range 4 {
		candidate := make([]string, n)
		for i := range candidate {
			candidate[i] = "nonexistent-arg"
		}
		if found.Args(found, candidate) == nil {
			return candidate
		}
	}
	t.Fatalf("could not find a valid positional arg count (0-3) for %q", found.CommandPath())
	return nil
}

// waitForTelemetryEventCount polls rec because the CLI sends telemetry from a
// background goroutine and os.Exit can race the HTTP request reaching the
// local mock server.
func waitForTelemetryEventCount(t *testing.T, rec *RecordedTelemetry, want int) []telemetry.UserEventInput {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		events := rec.Events()
		if len(events) >= want || time.Now().After(deadline) {
			return events
		}
		time.Sleep(20 * time.Millisecond)
	}
}
