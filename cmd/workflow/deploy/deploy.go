package deploy

import (
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	linkkey "github.com/smartcontractkit/cre-cli/cmd/account/link_key"
	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName  string `validate:"workflow_name"`
	WorkflowOwner string `validate:"workflow_owner"`
	WorkflowTag   string `validate:"omitempty,ascii,max=32"`
	DonFamily     string `validate:"required"`

	BinaryURL  string  `validate:"omitempty,http_url|eq="`
	ConfigURL  *string `validate:"omitempty,http_url|eq="`
	SecretsURL *string `validate:"omitempty,http_url|eq="`

	AutoStart bool

	KeepAlive    bool
	WorkflowPath string `validate:"required,file"`
	ConfigPath   string `validate:"omitempty,file,ascii,max=97" cli:"--config"`
	OutputPath   string `validate:"omitempty,filepath,ascii,max=97" cli:"--output"`

	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
}

func (i *Inputs) ResolveConfigURL(fallbackURL string) string {
	if i.ConfigURL == nil {
		return fallbackURL
	}
	return *i.ConfigURL
}

func (i *Inputs) ResolveSecretsURL(fallbackURL string) string {
	if i.SecretsURL == nil {
		return fallbackURL
	}
	return *i.SecretsURL
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

	validated bool
}

var defaultOutputPath = "./binary.wasm.br.b64"

func New(runtimeContext *runtime.Context) *cobra.Command {
	var deployCmd = &cobra.Command{
		Use:   "deploy <workflow-name>",
		Short: "Deploys a workflow to the Workflow Registry contract",
		Long:  `Compiles the workflow, uploads the artifacts, and registers the workflow in the Workflow Registry contract.`,
		Args:  cobra.ExactArgs(1),
		Example: `
		cre workflow deploy my-workflow
		`,
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

	settings.AddRawTxFlag(deployCmd)
	settings.AddSkipConfirmation(deployCmd)
	deployCmd.Flags().StringP("secrets-url", "s", "", "URL of the encrypted secrets JSON file")
	deployCmd.Flags().StringP("source-url", "x", "", "URL of the source code in Gist")
	deployCmd.Flags().StringP("output", "o", defaultOutputPath, "The output file for the compiled WASM binary encoded in base64")
	deployCmd.Flags().BoolP("config", "c", false, "Should include a config file (path defined in the workflow settings file) (default: false)")
	deployCmd.Flags().BoolP("keep-alive", "k", false, "Keep previous workflows with same workflow name and owner active (default: false).")
	deployCmd.Flags().BoolP("auto-start", "r", true, "Activate and run the workflow after registration, or pause it")

	return deployCmd
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	return &handler{
		log:              ctx.Logger,
		clientFactory:    ctx.ClientFactory,
		v:                ctx.Viper,
		settings:         ctx.Settings,
		stdin:            stdin,
		credentials:      ctx.Credentials,
		environmentSet:   ctx.EnvironmentSet,
		workflowArtifact: &workflowArtifact{},
		wrc:              nil,
		validated:        false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	var configURL *string
	if v.IsSet("config-url") {
		url := v.GetString("config-url")
		configURL = &url
	}

	var secretsURL *string
	if v.IsSet("secrets-url") {
		url := v.GetString("secrets-url")
		secretsURL = &url
	}

	return Inputs{
		WorkflowName:  h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner: h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowTag:   h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		ConfigURL:     configURL,
		SecretsURL:    secretsURL,
		AutoStart:     v.GetBool("auto-start"),
		DonFamily:     h.settings.Workflow.DevPlatformSettings.DonFamily,

		WorkflowPath: h.settings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		KeepAlive:    v.GetBool("keep-alive"),

		ConfigPath: h.settings.Workflow.WorkflowArtifactSettings.ConfigPath,
		OutputPath: v.GetString("output"),

		WorkflowRegistryContractChainName: h.environmentSet.WorkflowRegistryChainName,
		WorkflowRegistryContractAddress:   h.environmentSet.WorkflowRegistryAddress,
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
	if err := h.Compile(); err != nil {
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	if err := h.PrepareWorkflowArtifact(); err != nil {
		return fmt.Errorf("failed to prepare workflow artifact: %w", err)
	}

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("failed to create workflow registry client: %w", err)
	}
	h.wrc = wrc

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

	if err := h.UploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}
	if err := h.upsert(); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

func (h *handler) ensureOwnerLinkedOrFail() error {
	ownerAddr := common.HexToAddress(h.inputs.WorkflowOwner)

	linked, err := h.wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return fmt.Errorf("failed to check owner link status: %w", err)
	}

	h.log.Info().Str("owner", ownerAddr.Hex()).Bool("linked", linked).Msg("Workflow owner link status")

	if linked {
		return nil
	}

	h.log.Info().Str("owner", ownerAddr.Hex()).Msg("Owner not linked. Attempting auto-link...")
	if err := h.tryAutoLink(); err != nil {
		return fmt.Errorf("auto-link attempt failed: %w", err)
	}

	if linked, err = h.wrc.IsOwnerLinked(ownerAddr); err != nil {
		return fmt.Errorf("linked via auto-link, but failed to verify link status: %w", err)
	} else if !linked {
		return fmt.Errorf("auto-link executed but owner still not linked")
	}

	h.log.Info().Str("owner", ownerAddr.Hex()).Msg("Auto-link successful")
	return nil
}

func (h *handler) autoLinkMSIGAndExit() (halt bool, err error) {
	ownerAddr := common.HexToAddress(h.inputs.WorkflowOwner)

	linked, err := h.wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return false, fmt.Errorf("failed to check owner link status: %w", err)
	}

	if linked {
		h.log.Info().Str("owner", ownerAddr.Hex()).Msg("MSIG owner already linked. Continuing deploy.")
		return false, nil
	}

	h.log.Info().Str("owner", ownerAddr.Hex()).Bool("linked", linked).Msg("MSIG workflow owner link status")
	h.log.Info().Str("owner", ownerAddr.Hex()).Msg("MSIG owner: attempting auto-link...")

	if err := h.tryAutoLink(); err != nil {
		return false, fmt.Errorf("MSIG auto-link attempt failed: %w", err)
	}

	h.log.Info().Msg("MSIG auto-link initiated. Halting deploy. Submit the multisig transaction, then re-run deploy.")
	return true, nil
}

func (h *handler) tryAutoLink() error {
	rtx := &runtime.Context{
		Settings:       h.settings,
		Credentials:    h.credentials,
		ClientFactory:  h.clientFactory,
		Logger:         h.log,
		EnvironmentSet: h.environmentSet,
	}

	lkInputs := linkkey.Inputs{
		WorkflowOwner:                   h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowRegistryContractAddress: h.inputs.WorkflowRegistryContractAddress,
		WorkflowOwnerLabel:              "",
	}

	return linkkey.Exec(rtx, lkInputs)
}
