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

type workflowNameLookupClient interface {
	GetWorkflowListByOwnerAndName(owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
}

type userDonLimitChecker interface {
	CheckUserDonLimit(owner common.Address, donFamily string, pending uint32) error
}

func checkUserDonLimitBeforeDeploy(
	limitChecker userDonLimitChecker,
	nameLookup workflowNameLookupClient,
	owner common.Address,
	donFamily string,
	workflowName string,
	keepAlive bool,
	existingWorkflowStatus *uint8,
) error {
	if existingWorkflowStatus != nil {
		return nil
	}

	pending := uint32(1)
	if !keepAlive {
		activeSameName, err := countActiveWorkflowsByOwnerNameAndDON(nameLookup, owner, workflowName, donFamily)
		if err != nil {
			return fmt.Errorf("failed to check active workflows for %s on DON %s: %w", workflowName, donFamily, err)
		}
		if activeSameName >= pending {
			pending = 0
		} else {
			pending -= activeSameName
		}
	}

	if pending == 0 {
		return nil
	}

	return limitChecker.CheckUserDonLimit(owner, donFamily, pending)
}

func countActiveWorkflowsByOwnerNameAndDON(
	wrc workflowNameLookupClient,
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
