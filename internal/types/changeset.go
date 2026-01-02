package types

import (
	"github.com/smartcontractkit/chainlink/deployment/cre/workflow_registry/v2/changeset"
)

type ChangesetFile struct {
	Environment string      `json:"environment"`
	Domain      string      `json:"domain"`
	Changesets  []Changeset `json:"changesets"`
}

type Changeset struct {
	LinkOwner          *LinkOwner          `json:"LinkOwner,omitempty"`
	UnlinkOwner        *UnlinkOwner        `json:"UnlinkOwner,omitempty"`
	UpsertWorkflow     *UpsertWorkflow     `json:"UpsertWorkflow,omitempty"`
	BatchPauseWorkflow *BatchPauseWorkflow `json:"BatchPauseWorkflow,omitempty"`
	ActivateWorkflow   *ActivateWorkflow   `json:"ActivateWorkflow,omitempty"`
	DeleteWorkflow     *DeleteWorkflow     `json:"DeleteWorkflow,omitempty"`
}

type UpsertWorkflow struct {
	Payload changeset.UserWorkflowUpsertInput `json:"payload,omitempty"`
}

type LinkOwner struct {
	Payload changeset.UserLinkOwnerInput `json:"payload,omitempty"`
}

type UnlinkOwner struct {
	Payload changeset.UserUnlinkOwnerInput `json:"payload,omitempty"`
}

type BatchPauseWorkflow struct {
	Payload changeset.UserWorkflowBatchPauseInput `json:"payload,omitempty"`
}

type ActivateWorkflow struct {
	Payload changeset.UserWorkflowActivateInput `json:"payload,omitempty"`
}

type DeleteWorkflow struct {
	Payload changeset.UserWorkflowDeleteInput `json:"payload,omitempty"`
}
