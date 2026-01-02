package types

import (
	"fmt"
	commonconfig "github.com/smartcontractkit/chainlink-common/pkg/config"
	crecontracts "github.com/smartcontractkit/chainlink/deployment/cre/contracts"
	"github.com/smartcontractkit/chainlink/deployment/cre/workflow_registry/v2/changeset"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"time"
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

func WriteChangesetFile(fileName string, changesetFile *ChangesetFile, settings *settings.Settings) error {
	yamlData, err := yaml.Marshal(&changesetFile)
	if err != nil {
		return fmt.Errorf("failed to marshal changeset to yaml: %w", err)
	}

	fullFilePath := filepath.Join(
		filepath.Clean(settings.Workflow.CLDSettings.CLDPath),
		"domains",
		settings.Workflow.CLDSettings.Domain,
		settings.Workflow.CLDSettings.Environment,
		"durable_pipelines",
		"inputs",
		fileName,
	)
	if err := os.WriteFile(fullFilePath, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write changeset yaml file: %w", err)
	}

	fmt.Println("")
	fmt.Println("Changeset YAML file generated!")
	fmt.Printf("File: %s\n", fullFilePath)
	fmt.Println("")
	return nil
}

func MCMSConfig(settings *settings.Settings, chainSelector uint64) (*crecontracts.MCMSConfig, error) {
	minDelay, err := time.ParseDuration(settings.Workflow.CLDSettings.MCMSSettings.MinDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min delay duration: %w", err)
	}
	validDuration, err := time.ParseDuration(settings.Workflow.CLDSettings.MCMSSettings.ValidDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse valid duration: %w", err)
	}

	return &crecontracts.MCMSConfig{
		MinDelay:     minDelay,
		MCMSAction:   mcmstypes.TimelockActionSchedule,
		OverrideRoot: settings.Workflow.CLDSettings.MCMSSettings.OverrideRoot == "true",
		TimelockQualifierPerChain: map[uint64]string{
			chainSelector: settings.Workflow.CLDSettings.MCMSSettings.TimelockQualifier,
		},
		ValidDuration: commonconfig.MustNewDuration(validDuration),
	}, nil
}
