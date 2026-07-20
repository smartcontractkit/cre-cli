package workflowresolve

import (
	"context"
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
)

// ExecutionLookup resolves executions by on-chain id.
type ExecutionLookup interface {
	FindExecutionByOnChainID(ctx context.Context, onChainID string) (*workflowdataclient.Execution, error)
}

// ResolveExecutionUUID accepts a platform UUID or on-chain execution id and returns the platform UUID.
func ResolveExecutionUUID(ctx context.Context, wdc ExecutionLookup, arg string) (string, error) {
	if LooksLikeUUID(arg) {
		return arg, nil
	}
	if LooksLikeOnChainExecutionID(arg) {
		exec, err := wdc.FindExecutionByOnChainID(ctx, arg)
		if err != nil {
			return "", err
		}
		return exec.UUID, nil
	}
	return "", fmt.Errorf("%q is not a valid execution UUID or on-chain execution ID", arg)
}
