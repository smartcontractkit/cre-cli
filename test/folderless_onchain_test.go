package test

// Throwaway branch-verification test: folder-less pause/activate/delete against
// an ON-CHAIN workflow registry on a fresh local Anvil, resolving the workflow
// from a mocked CRE platform by name and by ID.

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	testcontracts "github.com/smartcontractkit/cre-cli/test/contracts"
)

func TestFolderlessOnchain(t *testing.T) {
	InitLogging()
	t.Setenv(environments.EnvVarEnv, "STAGING")
	// must match the DON limit configured by DeployTestWorkflowRegistry
	t.Setenv(environments.EnvVarDonFamily, "zone-a")

	// Fresh Anvil; dump state into a temp file so nothing in the repo is touched.
	anvilProc, anvilPort, err := StartAnvil(DUMP_ANVIL_STATE, filepath.Join(t.TempDir(), "anvil-dump.json"))
	require.NoError(t, err, "failed to start Anvil")
	defer StopAnvil(anvilProc)
	ethURL := "http://localhost:" + strconv.Itoa(anvilPort)

	sethClient := testcontracts.NewSethClientWithContracts(t, L, ethURL, constants.TestAnvilChainID, SethConfigPath)
	require.NotNil(t, sethClient, "seth client with contracts")
	registry, err := testcontracts.DeployTestWorkflowRegistry(t, sethClient)
	require.NoError(t, err, "failed to deploy workflow registry")
	registryAddr := registry.Address().Hex()

	// Register a workflow on-chain, owned by TestAddress3 (key index 2).
	wfName := "onchain-wf"
	wfIDHex, err := workflowUtils.GenerateWorkflowIDFromStrings(constants.TestAddress3, wfName, []byte("folderless-test-binary"), []byte{}, "")
	require.NoError(t, err)
	wfIDHex = strings.TrimPrefix(wfIDHex, "0x")
	wfIDBytes, err := hex.DecodeString(wfIDHex)
	require.NoError(t, err)
	var wfID [32]byte
	copy(wfID[:], wfIDBytes)

	_, err = sethClient.Decode(registry.UpsertWorkflow(
		sethClient.NewTXKeyOpts(2),
		wfName, "tag-folderless", wfID, 0 /* ACTIVE */, "zone-a",
		"https://example.test/binary.wasm", "", []byte{}, false,
	))
	require.NoError(t, err, "failed to register workflow on-chain")

	readStatus := func() uint8 {
		list, err := registry.GetWorkflowListByOwnerAndName(
			sethClient.NewCallOpts(), common.HexToAddress(constants.TestAddress3), wfName,
			big.NewInt(0), big.NewInt(10),
		)
		require.NoError(t, err)
		require.NotEmpty(t, list, "workflow should exist on-chain")
		return list[0].Status
	}
	require.Equal(t, uint8(0), readStatus(), "workflow should start ACTIVE")

	// Platform mock: no tenant registries; the row carries a
	// "contract:<selector>:<address>" source (fallback registry resolution).
	row := folderlessRow{
		uuid:   "1a2b3c4d-1111-4222-8333-444455556666",
		name:   wfName,
		id:     wfIDHex,
		owner:  strings.TrimPrefix(strings.ToLower(constants.TestAddress3), "0x"),
		status: "ACTIVE",
		source: fmt.Sprintf("contract:%d:%s", chainselectors.ANVIL_DEVNET.Selector, registryAddr),
	}
	mock := newFolderlessMockNoRegistries(t, []folderlessRow{row})
	gqlURL := mock.srv.URL + "/graphql"

	ownerKeyEnv := settings.EthPrivateKeyEnvVar + "=" + constants.TestPrivateKey3
	wrongKeyEnv := settings.EthPrivateKeyEnvVar + "=" + constants.TestPrivateKey2

	// --- error: no --rpc-url ---
	out, err := runFolderless(t, gqlURL, []string{ownerKeyEnv}, "workflow", "pause", wfName, "--yes")
	require.Error(t, err)
	require.Contains(t, out, "missing RPC URL for chain", out)
	require.Contains(t, out, "--rpc-url", out)

	// --- error: private key does not match owner ---
	out, err = runFolderless(t, gqlURL, []string{wrongKeyEnv}, "workflow", "pause", wfName, "--rpc-url", ethURL, "--yes")
	require.Error(t, err)
	require.Contains(t, out, "but CRE_ETH_PRIVATE_KEY derives", out)

	// --- pause by name ---
	out, err = runFolderless(t, gqlURL, []string{ownerKeyEnv}, "workflow", "pause", wfName, "--rpc-url", ethURL, "--yes")
	require.NoError(t, err, out)
	require.Contains(t, out, "Workflows paused successfully", out)
	require.Equal(t, uint8(1), readStatus(), "workflow should be PAUSED on-chain")

	// simulate platform indexer catching up
	mock.setStatus(wfIDHex, "PAUSED")

	// --- activate by ID ---
	out, err = runFolderless(t, gqlURL, []string{ownerKeyEnv}, "workflow", "activate", wfIDHex, "--rpc-url", ethURL, "--yes")
	require.NoError(t, err, out)
	require.Contains(t, out, "Workflow activated successfully", out)
	require.Equal(t, uint8(0), readStatus(), "workflow should be ACTIVE on-chain")

	mock.setStatus(wfIDHex, "ACTIVE")

	// --- delete by ID ---
	out, err = runFolderless(t, gqlURL, []string{ownerKeyEnv}, "workflow", "delete", wfIDHex, "--rpc-url", ethURL, "--yes")
	require.NoError(t, err, out)
	require.Contains(t, out, "Workflows deleted successfully", out)

	list, err := registry.GetWorkflowListByOwnerAndName(
		sethClient.NewCallOpts(), common.HexToAddress(constants.TestAddress3), wfName,
		big.NewInt(0), big.NewInt(10),
	)
	require.NoError(t, err)
	require.Empty(t, list, "workflow should be deleted on-chain")
}
