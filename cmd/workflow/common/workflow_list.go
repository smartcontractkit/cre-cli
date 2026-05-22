package common

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
)

const workflowListPageSize = int64(200)

// WorkflowListByOwnerAndNameClient fetches workflow metadata pages from the registry.
type WorkflowListByOwnerAndNameClient interface {
	GetWorkflowListByOwnerAndName(ctx context.Context, owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
}

// FetchAllWorkflowsByOwnerAndName returns every workflow version for owner+name, paginating until exhausted.
func FetchAllWorkflowsByOwnerAndName(
	ctx context.Context,
	wrc WorkflowListByOwnerAndNameClient,
	owner common.Address,
	name string,
) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	var (
		start     = big.NewInt(0)
		limit     = big.NewInt(workflowListPageSize)
		workflows = make([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, 0, workflowListPageSize)
	)

	for {
		list, err := wrc.GetWorkflowListByOwnerAndName(ctx, owner, name, start, limit)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}

		workflows = append(workflows, list...)

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < workflowListPageSize {
			break
		}
	}

	return workflows, nil
}
