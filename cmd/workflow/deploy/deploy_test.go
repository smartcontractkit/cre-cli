package deploy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestWorkflowDeployCommand(t *testing.T) {
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
					DonFamily:     "test_label",
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
					DonFamily:     "test_label",
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
					DonFamily:     "test_label",
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
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 0x12345",
			},
			{
				name: "Empty Workflow Owner",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: "",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.WorkflowOwner",
				wantDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: ",
			},
			{
				name: "Missing DonFamily",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
				},
				wantErr:    true,
				wantKey:    "Inputs.DonFamily",
				wantDetail: "DonFamily is a required field",
			},
			{
				name: "Invalid Binary URL",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
					BinaryURL:     "htp://invalid-url",
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.BinaryURL",
				wantDetail: "BinaryURL must be empty or a valid HTTP URL: htp://invalid-url",
			},
			{
				name: "Invalid Config URL",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
					BinaryURL:     "https://valid-url.com/binary",
					ConfigURL:     stringPtr("htp://invalid-url"),
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.ConfigURL",
				wantDetail: "ConfigURL must be empty or a valid HTTP URL: htp://invalid-url",
			},
			{
				name: "Invalid Secrets URL",
				inputs: Inputs{
					WorkflowName:  "valid_workflow",
					WorkflowOwner: chainsim.TestAddress,
					BinaryURL:     "https://valid-url.com/binary",
					SecretsURL:    stringPtr("htp://invalid-url"),
					DonFamily:     "test_label",
				},
				wantErr:    true,
				wantKey:    "Inputs.SecretsURL",
				wantDetail: "SecretsURL must be empty or a valid HTTP URL: htp://invalid-url",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()

				ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
				handler := newHandler(ctx, buf)

				ctx.Settings = createTestSettings(
					chainsim.TestAddress,
					"eoa",
					"test_workflow",
					"test_don_family",
					"testdata/basic_workflow/main.go",
					"",
				)
				handler.settings = ctx.Settings

				handler.inputs = tt.inputs
				err := handler.ValidateInputs()

				if tt.wantErr {
					require.Error(t, err)
					// Ensure the error is wrapped and of type ValidationErrors
					var verrs validation.ValidationErrors
					assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")

					validation.AssertValidationErrs(t, verrs, tt.wantKey, tt.wantDetail)
					assert.False(t, handler.validated)
				}
			})
		}
	})
}

func stringPtr(s string) *string {
	return &s
}
