package generate_id

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/artifacts"
	"github.com/smartcontractkit/cre-cli/internal/build"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const defaultOutputPath = "./binary.wasm.br.b64"

type Inputs struct {
	WorkflowPath  string `validate:"required,path_read"`
	WorkflowName  string `validate:"workflow_name"`
	WorkflowOwner string `validate:"workflow_owner" cli:"--owner"`

	ConfigPath string `validate:"omitempty,file,ascii,max=97" cli:"--config"`
	OutputPath string `validate:"omitempty,filepath,ascii,max=97" cli:"--output"`

	validated bool
}

type handler struct {
	log             *zerolog.Logger
	settings        *settings.Settings
	builder         *build.Builder
	artifactBuilder *artifacts.Builder

	inputs Inputs
}

func New(ctx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate-id <workflow-folder-path>",
		Short:   "Display the workflow ID",
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow generate-id ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(ctx)

			h.inputs = h.ResolveInputs(ctx.Viper)
			if err := h.ValidateInputs(); err != nil {
				return err
			}

			return h.Execute()
		},
	}

	// optional owner flag overrides the owner from settings
	cmd.Flags().String("owner", "", "Workflow owner address")
	cmd.Flags().StringP("output", "o", defaultOutputPath, "The output file for the compiled WASM binary encoded in base64")

	return cmd
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:             ctx.Logger,
		settings:        ctx.Settings,
		builder:         build.NewBuilder(ctx.Logger),
		artifactBuilder: artifacts.NewBuilder(ctx.Logger),
	}
}

func (h *handler) Execute() error {
	if !h.inputs.validated {
		return fmt.Errorf("inputs have not been validated")
	}

	buildParams, err := build.ResolveBuildParamsForWorkflow(h.inputs.WorkflowPath, h.inputs.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve build parameters: %w", err)
	}

	if err := h.builder.CompileAndSave(buildParams); err != nil {
		return fmt.Errorf("failed to compile workflow: %w", err)
	}

	workflowArtifact, err := h.artifactBuilder.Build(artifacts.Inputs{
		WorkflowOwner: h.inputs.WorkflowOwner,
		WorkflowName:  h.inputs.WorkflowName,
		OutputPath:    h.inputs.OutputPath,
		ConfigPath:    h.inputs.ConfigPath,
	})
	if err != nil {
		return fmt.Errorf("failed to build workflow artifact: %w", err)
	}

	h.log.Info().Str("workflowID", workflowArtifact.WorkflowID).Msg("Workflow ID computed successfully")
	fmt.Println("Workflow ID:", workflowArtifact.WorkflowID)

	return nil
}

func (h *handler) ResolveInputs(v *viper.Viper) Inputs {
	ownerFromFlag := v.GetString("owner")
	workflowOwner := h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress
	if ownerFromFlag != "" {
		workflowOwner = ownerFromFlag
	}

	return Inputs{
		WorkflowPath:  h.settings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		WorkflowName:  h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner: workflowOwner,

		ConfigPath: h.settings.Workflow.WorkflowArtifactSettings.ConfigPath,
		OutputPath: v.GetString("output"),
	}
}

func (h *handler) ValidateInputs() error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err := validate.Struct(h.inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}
	h.inputs.validated = true

	return nil
}
