package deploy

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName  string `validate:"workflow_name"`
	WorkflowOwner string `validate:"workflow_owner"`
	WorkflowTag   string `validate:"omitempty,ascii,max=32"`
	DonFamily     string `validate:"required"`

	BinaryURL string  `validate:"omitempty,http_url|eq="`
	ConfigURL *string `validate:"omitempty,http_url|eq="`

	KeepAlive    bool
	WorkflowPath string `validate:"required,path_read"`
	ConfigPath   string `validate:"omitempty,file,ascii,max=97" cli:"--config"`
	OutputPath   string `validate:"omitempty,filepath,ascii,max=97" cli:"--output"`

	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`

	OwnerLabel       string `validate:"omitempty"`
	SkipConfirmation bool
}

func (i *Inputs) ResolveConfigURL(fallbackURL string) string {
	if i.ConfigURL == nil {
		return fallbackURL
	}
	return *i.ConfigURL
}

type handler struct {
	log              *zerolog.Logger
	clientFactory    client.Factory
	v                *viper.Viper
	settings         *settings.Settings
	inputs           Inputs
	stdin            io.Reader
	credentials      *credentials.Credentials
	environmentSet   *environments.EnvironmentSet
	workflowArtifact *workflowArtifact
	wrc              *client.WorkflowRegistryV2Client
	runtimeContext   *runtime.Context

	validated bool

	// existingWorkflowStatus stores the status of an existing workflow when updating.
	// nil means this is a new workflow, otherwise it contains the current status (0=active, 1=paused).
	existingWorkflowStatus *uint8

	wg     sync.WaitGroup
	wrcErr error
}

var defaultOutputPath = "./binary.wasm.br.b64"

func New(runtimeContext *runtime.Context) *cobra.Command {
	var deployCmd = &cobra.Command{
		Use:     "deploy <workflow-folder-path>",
		Short:   "Deploys a workflow to the Workflow Registry contract",
		Long:    `Compiles the workflow, uploads the artifacts, and registers the workflow in the Workflow Registry contract.`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow deploy ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeContext, cmd.InOrStdin())

			inputs, err := h.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			h.inputs = inputs

			if err := h.ValidateInputs(); err != nil {
				return err
			}
			return h.Execute()
		},
	}

	settings.AddTxnTypeFlags(deployCmd)
	settings.AddSkipConfirmation(deployCmd)
	deployCmd.Flags().StringP("output", "o", defaultOutputPath, "The output file for the compiled WASM binary encoded in base64")
	deployCmd.Flags().StringP("owner-label", "l", "", "Label for the workflow owner (used during auto-link if owner is not already linked)")

	return deployCmd
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	h := handler{
		log:              ctx.Logger,
		clientFactory:    ctx.ClientFactory,
		v:                ctx.Viper,
		settings:         ctx.Settings,
		stdin:            stdin,
		credentials:      ctx.Credentials,
		environmentSet:   ctx.EnvironmentSet,
		workflowArtifact: &workflowArtifact{},
		wrc:              nil,
		runtimeContext:   ctx,
		validated:        false,
		wg:               sync.WaitGroup{},
		wrcErr:           nil,
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
	var configURL *string
	if v.IsSet("config-url") {
		url := v.GetString("config-url")
		configURL = &url
	}

	return Inputs{
		WorkflowName:  h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner: h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowTag:   h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		ConfigURL:     configURL,
		DonFamily:     h.environmentSet.DonFamily,

		WorkflowPath: h.settings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		KeepAlive:    false,

		ConfigPath: h.settings.Workflow.WorkflowArtifactSettings.ConfigPath,
		OutputPath: v.GetString("output"),

		WorkflowRegistryContractChainName: h.environmentSet.WorkflowRegistryChainName,
		WorkflowRegistryContractAddress:   h.environmentSet.WorkflowRegistryAddress,
		OwnerLabel:                        v.GetString("owner-label"),
		SkipConfirmation:                  v.GetBool(settings.Flags.SkipConfirmation.Name),
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
	h.displayWorkflowDetails()

	if err := h.Compile(); err != nil {
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	if err := h.PrepareWorkflowArtifact(); err != nil {
		return fmt.Errorf("failed to prepare workflow artifact: %w", err)
	}

	h.runtimeContext.Workflow.ID = h.workflowArtifact.WorkflowID

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
			// Ask for user confirmation before updating existing workflow
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
	ui.Dim("Uploading files...")
	if err := h.uploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}
	ui.Line()
	ui.Dim("Preparing deployment transaction...")
	if err := h.upsert(); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

func (h *handler) workflowExists() error {
	workflow, err := h.wrc.GetWorkflow(common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress), h.inputs.WorkflowName, h.inputs.WorkflowName)
	if err != nil {
		return err
	}
	if workflow.WorkflowId == [32]byte(common.Hex2Bytes(h.workflowArtifact.WorkflowID)) {
		return fmt.Errorf("workflow with id %s already exists", h.workflowArtifact.WorkflowID)

	}
	if workflow.WorkflowName == h.inputs.WorkflowName {
		status := workflow.Status
		h.existingWorkflowStatus = &status
		return fmt.Errorf("workflow with name %s already exists", h.inputs.WorkflowName)
	}
	return nil
}

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Deploying Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress))
	ui.Line()
}
