package deploy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/artifacts"
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

				wfArt := artifacts.Artifact{
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

func TestPrepareUpsertParams_StatusPreservation(t *testing.T) {
	t.Run("new workflow uses active status by default", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		handler := newHandler(ctx, buf)

		handler.inputs = Inputs{
			WorkflowName:                      "test_workflow",
			WorkflowOwner:                     chainsim.TestAddress,
			WorkflowPath:                      filepath.Join("testdata", "basic_workflow", "main.go"),
			DonFamily:                         "zone-a",
			WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryContractAddress:   simulatedEnvironment.Contracts.WorkflowRegistry.Contract.Hex(),
			BinaryURL:                         "https://example.com/binary",
			WorkflowTag:                       "test_tag",
		}
		handler.workflowArtifact = &artifacts.Artifact{
			BinaryData: []byte("0x1234"),
			ConfigData: []byte("config"),
			WorkflowID: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}
		handler.validated = true

		// No existing workflow status set (nil), so it should default to active (0)
		params, err := handler.prepareUpsertParams()
		require.NoError(t, err)
		assert.Equal(t, uint8(0), params.Status, "new workflow should have active status (0)")
	})

	t.Run("updating paused workflow preserves paused status", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		handler := newHandler(ctx, buf)

		handler.inputs = Inputs{
			WorkflowName:                      "test_workflow",
			WorkflowOwner:                     chainsim.TestAddress,
			WorkflowPath:                      filepath.Join("testdata", "basic_workflow", "main.go"),
			DonFamily:                         "zone-a",
			WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryContractAddress:   simulatedEnvironment.Contracts.WorkflowRegistry.Contract.Hex(),
			BinaryURL:                         "https://example.com/binary",
			WorkflowTag:                       "test_tag",
		}
		handler.workflowArtifact = &artifacts.Artifact{
			BinaryData: []byte("0x1234"),
			ConfigData: []byte("config"),
			WorkflowID: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}
		handler.validated = true

		// Simulate existing workflow with paused status (1)
		pausedStatus := uint8(1)
		handler.existingWorkflowStatus = &pausedStatus

		params, err := handler.prepareUpsertParams()
		require.NoError(t, err)
		assert.Equal(t, uint8(1), params.Status, "updating paused workflow should preserve paused status (1)")
	})

	t.Run("updating active workflow preserves active status", func(t *testing.T) {
		t.Parallel()
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		ctx, buf := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		handler := newHandler(ctx, buf)

		handler.inputs = Inputs{
			WorkflowName:                      "test_workflow",
			WorkflowOwner:                     chainsim.TestAddress,
			WorkflowPath:                      filepath.Join("testdata", "basic_workflow", "main.go"),
			DonFamily:                         "zone-a",
			WorkflowRegistryContractChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryContractAddress:   simulatedEnvironment.Contracts.WorkflowRegistry.Contract.Hex(),
			BinaryURL:                         "https://example.com/binary",
			WorkflowTag:                       "test_tag",
		}
		handler.workflowArtifact = &artifacts.Artifact{
			BinaryData: []byte("0x1234"),
			ConfigData: []byte("config"),
			WorkflowID: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}
		handler.validated = true

		// Simulate existing workflow with active status (0)
		activeStatus := uint8(0)
		handler.existingWorkflowStatus = &activeStatus

		params, err := handler.prepareUpsertParams()
		require.NoError(t, err)
		assert.Equal(t, uint8(0), params.Status, "updating active workflow should preserve active status (0)")
	})
}
