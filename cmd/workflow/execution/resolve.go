package execution

import (
	"context"
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// resolveExecutionUUID accepts either a platform UUID (8-4-4-4-12) or an
// on-chain hex execution ID (64-char hex shown in the Explorer UI) and returns
// the platform UUID needed for API calls.
func resolveExecutionUUID(ctx context.Context, wdc *workflowdataclient.Client, arg string) (string, error) {
	if looksLikeUUID(arg) {
		return arg, nil
	}
	if looksLikeOnChainExecutionID(arg) {
		spinner := ui.NewSpinner()
		spinner.Start(fmt.Sprintf("Resolving execution ID %q...", arg))
		exec, err := wdc.FindExecutionByOnChainID(ctx, arg)
		spinner.Stop()
		if err != nil {
			return "", err
		}
		return exec.UUID, nil
	}
	return "", fmt.Errorf("%q is not a valid execution UUID or on-chain execution ID", arg)
}

// looksLikeUUID returns true for the standard UUID shape (8-4-4-4-12).
func looksLikeUUID(s string) bool {
	parts := splitDash(s)
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lengths[i] {
			return false
		}
	}
	return true
}

// looksLikeOnChainExecutionID returns true for 64-char lowercase hex strings
// matching the execution id format shown in the Explorer UI.
func looksLikeOnChainExecutionID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func splitDash(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
