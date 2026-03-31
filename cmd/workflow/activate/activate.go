package activate

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	WorkflowStatusPaused = uint8(1)
)

type Inputs struct {
	WorkflowName                      string `validate:"workflow_name"`
	WorkflowOwner                     string `validate:"workflow_owner"`
	DonFamily                         string `validate:"required"`
	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	activateCmd := &cobra.Command{
		Use:     "activate <workflow-folder-path>",
		Short:   "Activates workflow on the Workflow Registry contract",
		Long:    `Changes workflow status to active on the Workflow Registry contract`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow activate ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			handler.inputs = inputs

			if err := handler.ValidateInputs(); err != nil {
				return err
			}
			return handler.Execute()
		},
	}

	settings.AddTxnTypeFlags(activateCmd)
	settings.AddSkipConfirmation(activateCmd)

	return activateCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	settings       *settings.Settings
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
	wrc            *client.WorkflowRegistryV2Client
	runtimeContext *runtime.Context

	validated bool

	wg     sync.WaitGroup
	wrcErr error
}

func newHandler(ctx *runtime.Context) *handler {
	h := handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		settings:       ctx.Settings,
		environmentSet: ctx.EnvironmentSet,
		runtimeContext: ctx,
		validated:      false,
		wg:             sync.WaitGroup{},
		wrcErr:         nil,
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
		if err != nil {
			h.wrcErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		h.wrc = wrc
	}()

	return &h
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		WorkflowName:                      h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:                     h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		DonFamily:                         h.environmentSet.DonFamily,
		WorkflowRegistryContractAddress:   h.environmentSet.WorkflowRegistryAddress,
		WorkflowRegistryContractChainName: h.environmentSet.WorkflowRegistryChainName,
	}, nil
}

func (h *handler) ValidateInputs() error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err := validate.Struct(h.inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	h.validated = true
	return nil
}

func (h *handler) Execute() error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	workflowName := h.inputs.WorkflowName
	workflowOwner := h.inputs.WorkflowOwner

	h.displayWorkflowDetails()

	h.wg.Wait()
	if h.wrcErr != nil {
		return h.wrcErr
	}

	ownerAddr := common.HexToAddress(workflowOwner)

	const pageLimit = 200
	workflows, err := h.wrc.GetWorkflowListByOwnerAndName(ownerAddr, workflowName, big.NewInt(0), big.NewInt(pageLimit))
	if err != nil {
		return fmt.Errorf("failed to get workflow list: %w", err)
	}
	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found for owner=%s name=%s", workflowOwner, workflowName)
	}

	// Sort by CreatedAt descending
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].CreatedAt > workflows[j].CreatedAt
	})

	latest := workflows[0]

	h.runtimeContext.Workflow.ID = hex.EncodeToString(latest.WorkflowId[:])

	// Validate precondition: workflow must be in paused state
	if latest.Status != WorkflowStatusPaused {
		return fmt.Errorf("workflow is already active, cancelling transaction")
	}

	if err := h.wrc.CheckUserDonLimit(ownerAddr, h.inputs.DonFamily, 1); err != nil {
		return err
	}

	ui.Dim(fmt.Sprintf("Activating workflow: Name=%s, Owner=%s, WorkflowID=%s", workflowName, workflowOwner, hex.EncodeToString(latest.WorkflowId[:])))

	txOut, err := h.wrc.ActivateWorkflow(latest.WorkflowId, h.inputs.DonFamily)
	if err != nil {
		return fmt.Errorf("failed to activate workflow: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		ui.Success(fmt.Sprintf("Transaction confirmed: %s", txOut.Hash))
		ui.URL(fmt.Sprintf("%s/tx/%s", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash))
		ui.Line()
		ui.Success("Workflow activated successfully")
		ui.Dim(fmt.Sprintf("   Contract address: %s", h.environmentSet.WorkflowRegistryAddress))
		ui.Dim(fmt.Sprintf("   Transaction hash: %s", txOut.Hash))
		ui.Dim(fmt.Sprintf("   Workflow Name:    %s", workflowName))
		ui.Dim(fmt.Sprintf("   Workflow ID:      %s", hex.EncodeToString(latest.WorkflowId[:])))

	case client.Raw:
		ui.Line()
		ui.Success("MSIG workflow activation transaction prepared!")
		ui.Dim(fmt.Sprintf("To Activate %s with workflowID: %s", workflowName, hex.EncodeToString(latest.WorkflowId[:])))
		ui.Line()
		ui.Bold("Next steps:")
		ui.Line()
		ui.Print("   1. Submit the following transaction on the target chain:")
		ui.Dim(fmt.Sprintf("      Chain:            %s", h.inputs.WorkflowRegistryContractChainName))
		ui.Dim(fmt.Sprintf("      Contract Address: %s", txOut.RawTx.To))
		ui.Line()
		ui.Print("   2. Use the following transaction data:")
		ui.Line()
		ui.Code(fmt.Sprintf("      %x", txOut.RawTx.Data))
		ui.Line()

	case client.Changeset:
		chainSelector, err := settings.GetChainSelectorByChainName(h.environmentSet.WorkflowRegistryChainName)
		if err != nil {
			return fmt.Errorf("failed to get chain selector for chain %q: %w", h.environmentSet.WorkflowRegistryChainName, err)
		}
		mcmsConfig, err := settings.GetMCMSConfig(h.settings, chainSelector)
		if err != nil {
			ui.Warning("MCMS config not found or is incorrect, skipping MCMS config in changeset")
		}
		cldSettings := h.settings.CLDSettings
		changesets := []types.Changeset{
			{
				ActivateWorkflow: &types.ActivateWorkflow{
					Payload: types.UserWorkflowActivateInput{
						WorkflowID: h.runtimeContext.Workflow.ID,
						DonFamily:  h.inputs.DonFamily,

						ChainSelector:             chainSelector,
						MCMSConfig:                mcmsConfig,
						WorkflowRegistryQualifier: cldSettings.WorkflowRegistryQualifier,
					},
				},
			},
		}
		csFile := types.NewChangesetFile(cldSettings.Environment, cldSettings.Domain, cldSettings.MergeProposals, changesets)

		var fileName string
		if cldSettings.ChangesetFile != "" {
			fileName = cldSettings.ChangesetFile
		} else {
			fileName = fmt.Sprintf("ActivateWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
		}

		return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
}

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Activating Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress))
	ui.Line()
}
