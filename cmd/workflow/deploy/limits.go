package deploy

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
)

const (
	workflowStatusActive = uint8(0)
	workflowListPageSize = int64(200)
)

type userDonLimitClient interface {
	GetMaxWorkflowsPerUserDONByFamily(user common.Address, donFamily string) (uint32, error)
	GetWorkflowListByOwner(owner common.Address, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
	GetWorkflowListByOwnerAndName(owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
}

func checkUserDonLimitBeforeDeploy(
	wrc userDonLimitClient,
	owner common.Address,
	donFamily string,
	workflowName string,
	keepAlive bool,
	existingWorkflowStatus *uint8,
) error {
	if existingWorkflowStatus != nil {
		return nil
	}

	maxAllowed, err := wrc.GetMaxWorkflowsPerUserDONByFamily(owner, donFamily)
	if err != nil {
		return fmt.Errorf("failed to fetch per-user workflow limit: %w", err)
	}

	currentActive, err := countActiveWorkflowsByOwnerAndDON(wrc, owner, donFamily)
	if err != nil {
		return fmt.Errorf("failed to check active workflows for DON %s: %w", donFamily, err)
	}

	effectiveActive := currentActive
	if !keepAlive {
		activeSameName, err := countActiveWorkflowsByOwnerNameAndDON(wrc, owner, workflowName, donFamily)
		if err != nil {
			return fmt.Errorf("failed to check active workflows for %s on DON %s: %w", workflowName, donFamily, err)
		}
		if activeSameName > effectiveActive {
			activeSameName = effectiveActive
		}
		effectiveActive -= activeSameName
	}

	if effectiveActive+1 > maxAllowed {
		return fmt.Errorf("workflow limit reached for DON %s: %d/%d active workflows", donFamily, effectiveActive, maxAllowed)
	}

	return nil
}

func countActiveWorkflowsByOwnerAndDON(
	wrc userDonLimitClient,
	owner common.Address,
	donFamily string,
) (uint32, error) {
	var count uint32
	start := big.NewInt(0)
	limit := big.NewInt(workflowListPageSize)

	for {
		list, err := wrc.GetWorkflowListByOwner(owner, start, limit)
		if err != nil {
			return 0, err
		}
		if len(list) == 0 {
			break
		}

		for _, workflow := range list {
			if workflow.Status == workflowStatusActive && workflow.DonFamily == donFamily {
				count++
			}
		}

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < workflowListPageSize {
			break
		}
	}

	return count, nil
}

func countActiveWorkflowsByOwnerNameAndDON(
	wrc userDonLimitClient,
	owner common.Address,
	workflowName string,
	donFamily string,
) (uint32, error) {
	var count uint32
	start := big.NewInt(0)
	limit := big.NewInt(workflowListPageSize)

	for {
		list, err := wrc.GetWorkflowListByOwnerAndName(owner, workflowName, start, limit)
		if err != nil {
			return 0, err
		}
		if len(list) == 0 {
			break
		}

		for _, workflow := range list {
			if workflow.Status == workflowStatusActive && workflow.DonFamily == donFamily {
				count++
			}
		}

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < workflowListPageSize {
			break
		}
	}

	return count, nil
}
