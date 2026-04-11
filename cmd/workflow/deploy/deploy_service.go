package deploy

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// deployService abstracts target-specific deployment logic. Implementations
// handle update confirmation, artifact upload, and registry upsert for their
// respective registry targets.
type deployService interface {
	Deploy() error
}

func newDeployService(target registryTarget, h *handler) deployService {
	if target.isPrivate() {
		return &privateDeployService{h: h}
	}
	return &onchainDeployService{h: h}
}

// onchainDeployService deploys workflows to the onchain workflow registry.
type onchainDeployService struct {
	h *handler
}

func (s *onchainDeployService) Deploy() error {
	h := s.h

	h.wg.Wait()
	if h.wrcErr != nil {
		return h.wrcErr
	}

	ui.Line()
	ui.Dim("Verifying ownership...")
	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		halt, err := h.autoLinkMSIGAndExit()
		if err != nil {
			return fmt.Errorf("failed to check/handle MSIG owner link status: %w", err)
		}
		if halt {
			return nil
		}
	} else {
		if err := h.ensureOwnerLinkedOrFail(); err != nil {
			return err
		}
	}

	existsErr := h.workflowExists()
	if existsErr != nil {
		if existsErr.Error() == "workflow with name "+h.inputs.WorkflowName+" already exists" {
			ui.Warning(fmt.Sprintf("Workflow %s already exists", h.inputs.WorkflowName))
			ui.Dim("This will update the existing workflow.")
			if !h.inputs.SkipConfirmation {
				confirm, err := ui.Confirm("Are you sure you want to overwrite the workflow?")
				if err != nil {
					return err
				}
				if !confirm {
					return errors.New("deployment cancelled by user")
				}
			}
		} else {
			return existsErr
		}
	}

	ui.Line()
	ui.Dim("Uploading files...")
	if err := h.uploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}

	if err := checkUserDonLimitBeforeDeploy(
		h.wrc,
		h.wrc,
		common.HexToAddress(h.inputs.WorkflowOwner),
		h.inputs.DonFamily,
		h.inputs.WorkflowName,
		h.inputs.KeepAlive,
		h.existingWorkflowStatus,
	); err != nil {
		return err
	}

	ui.Line()
	ui.Dim("Preparing deployment transaction...")
	if err := h.upsert(); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

// privateDeployService deploys workflows to the private workflow registry
// via GraphQL. Ownership linking and onchain prechecks are not applicable.
type privateDeployService struct {
	h *handler
}

func (s *privateDeployService) Deploy() error {
	h := s.h

	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	// TODO: check if workflow already exists in private registry and confirm update

	ui.Line()
	ui.Dim("Uploading files...")
	if err := h.uploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}

	input := h.buildPrivateRegistryInput()

	gql := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	prc := privateregistryclient.New(gql, h.log)

	ui.Line()
	ui.Dim(fmt.Sprintf("Registering workflow in private registry (workflowID: %s)...", input.WorkflowID))

	result, err := prc.UpsertWorkflowInRegistry(input)
	if err != nil {
		return fmt.Errorf("failed to register workflow in private registry: %w", err)
	}

	ui.Success("Workflow registered in private registry")
	ui.Line()
	ui.Bold("Details:")
	ui.Dim(fmt.Sprintf("   Workflow Name: %s", result.WorkflowName))
	ui.Dim(fmt.Sprintf("   Workflow ID:   %s", result.WorkflowID))
	ui.Dim(fmt.Sprintf("   Status:        %s", result.Status))
	ui.Dim(fmt.Sprintf("   Binary URL:    %s", result.BinaryURL))
	if result.ConfigURL != "" {
		ui.Dim(fmt.Sprintf("   Config URL:    %s", result.ConfigURL))
	}
	if result.Owner != "" {
		ui.Dim(fmt.Sprintf("   Owner:         %s", result.Owner))
	}

	return nil
}
