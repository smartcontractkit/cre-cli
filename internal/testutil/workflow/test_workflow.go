package workflowtest

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func RegisterWorkflow(t *testing.T, wrc *client.WorkflowRegistryV2Client, workflowName string, keepAlive bool) {
	workflowBytes := []byte("0x1234")
	workflowID, err := workflowUtils.GenerateWorkflowIDFromStrings(chainsim.TestAddress, workflowName, workflowBytes, []byte{}, "")
	require.NoError(t, err)

	params := client.RegisterWorkflowV2Parameters{
		WorkflowName: workflowName,
		WorkflowID:   [32]byte(common.Hex2Bytes(workflowID)),
		BinaryURL:    "https://example.com/binary",
		ConfigURL:    "",
		KeepAlive:    keepAlive,
		DonFamily:    "1",
	}

	err = wrc.UpsertWorkflow(params)
	require.NoError(t, err, "Failed to register workflow")
}
