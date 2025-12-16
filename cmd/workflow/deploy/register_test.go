package deploy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/artifact"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestWorkflowUpsert(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name       string
			inputs     Inputs
			wantErr    bool
			wantKey    string
			wantDetail string
		}{
			{
				name: "Valid Inputs",
				inputs: Inputs{
					WorkflowName:                      "test_workflow",
					WorkflowOwner:                     chainsim.TestAddress,
					WorkflowPath:                      filepath.Join("testdata", "basic_workflow", "main.go"),
					ConfigPath:                        filepath.Join("testdata", "basic_workflow", "config.yml"),
					DonFamily:                         "zone-a",
					WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
					BinaryURL:                         "https://example.com/binary",
					KeepAlive:                         true,
					ConfigURL:                         nil,
					WorkflowTag:                       "test_tag",
				},
				wantErr:    false,
				wantKey:    "",
				wantDetail: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
				defer simulatedEnvironment.Close()

				ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
				handler := newHandler(ctx, buf)
				tt.inputs.WorkflowRegistryContractAddress = simulatedEnvironment.Contracts.WorkflowRegistry.Contract.Hex()

				wrc, err := handler.clientFactory.NewWorkflowRegistryV2Client()
				require.NoError(t, err)
				handler.wrc = wrc

				handler.inputs = tt.inputs
				err = handler.ValidateInputs()
				require.NoError(t, err)

				wfArt := artifact.Artifact{
					BinaryData: []byte("0x1234"),
					ConfigData: []byte("config"),
					WorkflowID: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				}

				handler.workflowArtifact = &wfArt

				err = handler.upsert()
				require.NoError(t, err)
			})
		}
	})
}
