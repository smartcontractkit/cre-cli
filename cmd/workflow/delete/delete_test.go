package delete

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
	"github.com/smartcontractkit/dev-platform/internal/testutil"
	"github.com/smartcontractkit/dev-platform/internal/testutil/chainsim"
	"github.com/smartcontractkit/dev-platform/internal/validation"
)

func TestWorkflowDeleteCommand(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			inputs     Inputs
			wantErr    bool
			wantKey    string
			wantDetail string
		}{
			{
				name: "Empty Workflow Name",
				inputs: Inputs{
					WorkflowName:  "",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty and no longer than 64 characters: ",
			},
			{
				name: "Workflow Name Too Long",
				inputs: Inputs{
					WorkflowName:  "this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
					WorkflowOwner: "0x4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowName",
				wantDetail: "WorkflowName must be non-empty and no longer than 64 characters: this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			{
				name: "Invalid Workflow Owner - Missing 0x Prefix",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 4c0883a69102937d6234146e38a6aefbf95944c6e4d4013a6d287e4d739e7f9b",
			},
			{
				name: "Invalid Workflow Owner - Wrong Length",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "0x12345",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 0x12345",
			},
			{
				name: "Empty Workflow Owner",
				inputs: Inputs{
					WorkflowName: "valid_workflow",
				},
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
						EthPrivateKey: chainsim.TestPrivateKey,
					},
				}
				ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType = constants.WorkflowOwnerTypeEOA

				if tt.inputs.WorkflowRegistryContractAddress == "" {
					tt.inputs.WorkflowRegistryContractAddress = "0x0000000000000000000000000000000000000000"
				}
				if tt.inputs.WorkflowRegistryContractChainselector == 0 {
					tt.inputs.WorkflowRegistryContractChainselector = 11155111
				}

				handler := newHandler(ctx, testutil.EmptyMockStdinReader())
				handler.inputs = tt.inputs

				err := handler.ValidateInputs()

				if tt.wantErr {
					require.Error(t, err)
					var verrs validation.ValidationErrors
					assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")
					validation.AssertValidationErrs(t, verrs, tt.wantKey, tt.wantDetail)
					assert.False(t, handler.validated)
				} else {
					require.NoError(t, err)
					assert.True(t, handler.validated)
				}
			})
		}
	})
}
