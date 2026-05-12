package pause

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestNonInteractive_WithoutYes_ReturnsError(t *testing.T) {
	t.Parallel()
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	defer simulatedEnvironment.Close()
	ctx := simulatedEnvironment.NewRuntimeContext()
	ctx.Settings = &settings.Settings{
		User: settings.UserSettings{
			PrivateKeys: map[string]string{settings.EVM.Name: chainsim.TestPrivateKey},
		},
	}
	ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = constants.WorkflowOwnerTypeEOA

	h := newHandler(ctx)
	h.inputs = Inputs{
		WorkflowName:     "test-workflow",
		WorkflowOwner:    chainsim.TestAddress,
		NonInteractive:   true,
		SkipConfirmation: false,
	}
	h.validated = true

	err := h.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required flags for --non-interactive mode")
}

func TestNonInteractive_WithYes_PassesGuard(t *testing.T) {
	t.Parallel()
	simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
	defer simulatedEnvironment.Close()
	ctx := simulatedEnvironment.NewRuntimeContext()
	ctx.Settings = &settings.Settings{
		User: settings.UserSettings{
			PrivateKeys: map[string]string{settings.EVM.Name: chainsim.TestPrivateKey},
		},
	}
	ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = constants.WorkflowOwnerTypeEOA

	h := newHandler(ctx)
	h.inputs = Inputs{
		WorkflowName:     "test-workflow",
		WorkflowOwner:    chainsim.TestAddress,
		NonInteractive:   true,
		SkipConfirmation: true,
	}
	h.validated = true

	err := h.Execute()
	// Guard passes; error comes from WRC (no matching workflow), not the guard
	require.Error(t, err)
	require.NotContains(t, err.Error(), "missing required flags for --non-interactive mode")
}

func TestWorkflowPauseCommand(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()

		validRequired := func(in Inputs) Inputs {
			return in
		}

		tests := []struct {
			name       string
			inputs     Inputs
			wantErr    bool
			wantKey    string
			wantDetail string
		}{
			{
				name: "Empty Workflow Name",
				inputs: validRequired(Inputs{
					WorkflowName:  "",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				}),
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): ",
			},
			{
				name: "Workflow Name Too Long",
				inputs: validRequired(Inputs{
					WorkflowName:  "this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				}),
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			{
				name: "Invalid Workflow Owner - Missing 0x Prefix",
				inputs: validRequired(Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				}),
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
			},
			{
				name: "Invalid Workflow Owner - Wrong Length",
				inputs: validRequired(Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "0x12345",
				}),
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 0x12345",
			},
			{
				name: "Empty Workflow Owner",
				inputs: validRequired(Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "",
				}),
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: ",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()
				ctx := simulatedEnvironment.NewRuntimeContext()
				ctx.Settings = &settings.Settings{
					User: settings.UserSettings{
						PrivateKeys: map[string]string{settings.EVM.Name: chainsim.TestPrivateKey},
					},
				}
				ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = constants.WorkflowOwnerTypeEOA

				h := newHandler(ctx)
				h.inputs = tt.inputs
				err := h.ValidateInputs()

				if tt.wantErr {
					require.Error(t, err)
					var verrs validation.ValidationErrors
					assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")

					validation.AssertValidationErrs(t, verrs, tt.wantKey, tt.wantDetail)
					assert.False(t, h.validated)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
