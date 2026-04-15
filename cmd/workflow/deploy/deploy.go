package deploy

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/accessrequest"
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
	WorkflowPath string `validate:"required,workflow_path_read"`
	ConfigPath   string `validate:"omitempty,file,ascii,max=2048" cli:"--config"`
	OutputPath   string `validate:"omitempty,filepath,ascii,max=97" cli:"--output"`
	WasmPath     string `validate:"omitempty,file,ascii,max=2048" cli:"--wasm"`

	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`

	OwnerLabel       string `validate:"omitempty"`
	SkipConfirmation bool
	// SkipTypeChecks passes --skip-type-checks to cre-compile for TypeScript workflows.
	SkipTypeChecks bool

	PreviewPrivateRegistry bool
	TargetWorkflowRegistry registryTarget
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
	accessRequester  *accessrequest.Requester
	validated        bool

	// URL-fetched data for WorkflowID computation when --wasm or --config are URLs.
	urlBinaryData []byte
	urlConfigData []byte

	// existingWorkflowStatus stores the status of an existing workflow when updating.
	// nil means this is a new workflow, otherwise it contains the current status (0=active, 1=paused).
	existingWorkflowStatus *uint8
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
			return h.Execute(cmd.Context())
		},
	}

	settings.AddTxnTypeFlags(deployCmd)
	settings.AddSkipConfirmation(deployCmd)
	deployCmd.Flags().StringP("output", "o", defaultOutputPath, "The output file for the compiled WASM binary encoded in base64")
	deployCmd.Flags().StringP("owner-label", "l", "", "Label for the workflow owner (used during auto-link if owner is not already linked)")
	deployCmd.Flags().String("wasm", "", "Path to a pre-built WASM binary (skips compilation)")
	deployCmd.Flags().String("config", "", "Override the config file path from workflow.yaml")
	deployCmd.Flags().Bool("no-config", false, "Deploy without a config file")
	deployCmd.Flags().Bool("default-config", false, "Use the config path from workflow.yaml settings (default behavior)")
	deployCmd.Flags().Bool(cmdcommon.SkipTypeChecksCLIFlag, false, "Skip TypeScript project typecheck during compilation (passes "+cmdcommon.SkipTypeChecksFlag+" to cre-compile)")
	deployCmd.Flags().Bool("preview-private-registry", false, "Deploy to the private workflow registry (unreleased feature)")
	deployCmd.MarkFlagsMutuallyExclusive("config", "no-config", "default-config")

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
		runtimeContext:   ctx,
		accessRequester:  accessrequest.NewRequester(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger),
	}

	return &h
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	var configURL *string
	if v.IsSet("config-url") {
		url := v.GetString("config-url")
		configURL = &url
	}
	previewPrivateRegistry := v.GetBool("preview-private-registry")
	targetWorkflowRegistry, err := resolveRegistryTarget(previewPrivateRegistry, h.environmentSet)
	if err != nil {
		return Inputs{}, err
	}
	resolvedWorkflowOwner, err := h.resolveWorkflowOwner(targetWorkflowRegistry)
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve workflow owner: %w", err)
	}

	workflowTag := h.settings.Workflow.UserWorkflowSettings.WorkflowName
	if len(workflowTag) > 32 {
		workflowTag = workflowTag[:32]
	}

	return Inputs{
		WorkflowName:  h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner: resolvedWorkflowOwner,
		WorkflowTag:   workflowTag,
		ConfigURL:     configURL,
		DonFamily:     h.environmentSet.DonFamily,

		WorkflowPath: h.settings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		KeepAlive:    false,

		ConfigPath: cmdcommon.ResolveConfigPath(v, h.settings.Workflow.WorkflowArtifactSettings.ConfigPath),
		OutputPath: v.GetString("output"),
		WasmPath:   v.GetString("wasm"),

		WorkflowRegistryContractChainName: h.environmentSet.WorkflowRegistryChainName,
		WorkflowRegistryContractAddress:   h.environmentSet.WorkflowRegistryAddress,
		OwnerLabel:                        v.GetString("owner-label"),
		SkipConfirmation:                  v.GetBool(settings.Flags.SkipConfirmation.Name),
		SkipTypeChecks:                    v.GetBool(cmdcommon.SkipTypeChecksCLIFlag),
		PreviewPrivateRegistry:            previewPrivateRegistry,
		TargetWorkflowRegistry:            targetWorkflowRegistry,
	}, nil
}

func (h *handler) ValidateInputs() error {
	// URLs bypass the struct-level file/ascii/max validators.
	wasmIsURL := cmdcommon.IsURL(h.inputs.WasmPath)
	configIsURL := cmdcommon.IsURL(h.inputs.ConfigPath)
	savedWasm := h.inputs.WasmPath
	savedConfig := h.inputs.ConfigPath
	if wasmIsURL {
		h.inputs.WasmPath = ""
	}
	if configIsURL {
		h.inputs.ConfigPath = ""
	}

	validate, err := validation.NewValidator()
	if err != nil {
		h.inputs.WasmPath = savedWasm
		h.inputs.ConfigPath = savedConfig
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err := validate.Struct(h.inputs); err != nil {
		h.inputs.WasmPath = savedWasm
		h.inputs.ConfigPath = savedConfig
		return validate.ParseValidationErrors(err)
	}

	h.inputs.WasmPath = savedWasm
	h.inputs.ConfigPath = savedConfig

	h.validated = true
	return nil
}

func (h *handler) Execute(ctx context.Context) error {
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		return fmt.Errorf("failed to check deployment access: %w", err)
	}

	if !deployAccess.HasAccess {
		return h.accessRequester.PromptAndSubmitRequest(ctx)
	}

	adapter := newRegistryDeployStrategy(h.inputs.TargetWorkflowRegistry, h)

	if err := h.prepareArtifacts(); err != nil {
		return err
	}

	if err := adapter.RunPreDeployChecks(); err != nil {
		if errors.Is(err, errDeployHalted) {
			return nil
		}
		return err
	}

	exists, err := adapter.CheckWorkflowExists()
	if err != nil {
		return fmt.Errorf("failed to check if workflow exists: %w", err)
	}
	if exists {
		if err := confirmWorkflowOverwrite(h.inputs.WorkflowName, h.inputs.SkipConfirmation); err != nil {
			return err
		}
	}

	ui.Line()
	ui.Dim("Uploading files...")
	if err := h.uploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}

	return adapter.Upsert()
}

// prepareArtifacts handles compile/fetch, artifact preparation, and hashing.
// Artifact upload is deferred to the deploy service so it runs after any
// existing-workflow update confirmation.
func (h *handler) prepareArtifacts() error {
	h.displayWorkflowDetails()

	if cmdcommon.IsURL(h.inputs.WasmPath) {
		h.inputs.BinaryURL = h.inputs.WasmPath
		ui.Dim("Fetching binary from URL for workflow ID computation...")
		fetched, err := cmdcommon.FetchURL(h.inputs.WasmPath)
		if err != nil {
			return fmt.Errorf("failed to fetch binary from URL: %w", err)
		}
		h.urlBinaryData = fetched
		ui.Success(fmt.Sprintf("Using binary URL: %s", h.inputs.WasmPath))
	} else {
		if err := h.Compile(); err != nil {
			return fmt.Errorf("failed to compile workflow: %w", err)
		}
	}

	if cmdcommon.IsURL(h.inputs.ConfigPath) {
		url := h.inputs.ConfigPath
		h.inputs.ConfigURL = &url
		h.inputs.ConfigPath = ""
		ui.Dim("Fetching config from URL for workflow ID computation...")
		fetched, err := cmdcommon.FetchURL(url)
		if err != nil {
			return fmt.Errorf("failed to fetch config from URL: %w", err)
		}
		h.urlConfigData = fetched
		ui.Success(fmt.Sprintf("Using config URL: %s", url))
	}

	if err := h.PrepareWorkflowArtifact(h.inputs.WorkflowOwner); err != nil {
		return fmt.Errorf("failed to prepare workflow artifact: %w", err)
	}

	ui.Dim(fmt.Sprintf("Binary hash:   %s", cmdcommon.HashBytes(h.workflowArtifact.RawBinaryForID)))
	ui.Dim(fmt.Sprintf("Config hash:   %s", cmdcommon.HashBytes(h.workflowArtifact.RawConfigForID)))
	ui.Dim(fmt.Sprintf("Workflow hash: %s", h.workflowArtifact.WorkflowID))

	h.runtimeContext.Workflow.ID = h.workflowArtifact.WorkflowID

	return nil
}

// resolveWorkflowOwner returns the effective owner address for workflow ID computation.
// For private registry deploys, the owner is derived from tenantID and organizationID.
// For onchain deploys, the configured WorkflowOwner address is used directly.
func (h *handler) resolveWorkflowOwner(targetWorkflowRegistry registryTarget) (string, error) {
	if !targetWorkflowRegistry.isPrivate() {
		return h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, nil
	}

	if h.runtimeContext.TenantContext == nil {
		return "", fmt.Errorf("tenant context is required for private registry deployment")
	}

	tenantID := h.runtimeContext.TenantContext.TenantID
	if tenantID == "" {
		return "", fmt.Errorf("tenant ID is required for private registry deployment")
	}

	orgID, err := h.credentials.GetOrgID()
	if err != nil {
		return "", fmt.Errorf("failed to get organization ID for private registry deployment: %w", err)
	}

	ownerBytes, err := workflowUtils.GenerateWorkflowOwnerAddress(tenantID, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to derive workflow owner address: %w", err)
	}

	return "0x" + hex.EncodeToString(ownerBytes), nil
}

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Deploying Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.inputs.WorkflowOwner))
	ui.Line()
}
