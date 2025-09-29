package test

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	test "github.com/smartcontractkit/cre-cli/test/contracts"
)

func initGenerateEnv(t *testing.T) (*os.Process, string) {
	InitLogging()

	err := os.Remove("anvil-state.json")
	if err != nil {
		require.NoError(t, err, "Not able to remove old Anvil state file")
	}

	// this time we are dumping state, not loading it
	anvilProc, anvilPort, err := StartAnvil(DUMP_ANVIL_STATE, "anvil-state.json")
	require.NoError(t, err, "Failed to start Anvil")
	ethUrl := "http://localhost:" + strconv.Itoa(anvilPort)
	return anvilProc, ethUrl
}

func initGenerateEnvForSimulator(t *testing.T) (*os.Process, string) {
	InitLogging()

	err := os.Remove("anvil-state-simulator.json")
	if err != nil {
		require.NoError(t, err, "Not able to remove old Anvil state file for simulator")
	}

	// this time we are dumping state, not loading it
	anvilProc, anvilPort, err := StartAnvil(DUMP_ANVIL_STATE, "anvil-state-simulator.json")
	require.NoError(t, err, "Failed to start Anvil for simulator")
	ethUrl := "http://localhost:" + strconv.Itoa(anvilPort)
	return anvilProc, ethUrl
}

func stopAnvilGracefully(anvilProc *os.Process) {
	// Set up a signal handler to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Send SIGTERM to Anvil process
	if err := anvilProc.Signal(syscall.SIGTERM); err != nil {
		L.Error().Err(err).Msgf("Error sending SIGTERM")
	}

	// this is just to leave enough time for Anvil to store state
	time.Sleep(5 * time.Second)

	// check if Anvil has exited gracefully, if not, then state dump failed
	_, err := anvilProc.Wait()
	if err != nil {
		L.Error().Err(err).Msg("Error waiting for Anvil to exit, could be still running in background, please terminate with SIGTERM")
	} else {
		L.Info().Msg("Anvil exited gracefully")
	}
}

// NOTE: this is not really a test, this is a script to re-generated Anvil EVM state dump
// once generated, state dump will contain pre-baked contracts and contract setups
// it will help with running tests
// please enable this test (comment out first line "t.Skip") and run it as standalone test only when necessary:
//
//	go test -run ^TestGenerateAnvilState$ -v
//
// the reason why it's created as a test rather than as a script is to be able to reuse
// multiple helper functions available in test files
func TestGenerateAnvilState(t *testing.T) {
	t.Skip("Re-enable this test only when it's required to re-generate Anvil state dump")

	anvilProc, testEthUrl := initGenerateEnv(t)
	defer stopAnvilGracefully(anvilProc)

	tc := NewTestConfig(t)
	t.Cleanup(tc.Cleanup(t))

	// deploy and config contracts on Anvil
	sethClient := test.NewSethClientWithContracts(t, L, testEthUrl, constants.TestAnvilChainID, SethConfigPath)
	_, err := test.DeployTestWorkflowRegistry(t, sethClient)
	if err != nil {
		t.Fatalf("failed to deploy and configure WorkflowRegistry: %v", err)
	}
}

// NOTE: this is not really a test, this is a script to re-generated Anvil EVM state dump
// once generated, state dump will contain pre-baked contracts and contract setups
// it will help with running tests
// please enable this test (comment out first line "t.Skip") and run it as standalone test only when necessary:
//
//	go test -run ^TestGenerateAnvilStateForSimulator$ -v
//
// the reason why it's created as a test rather than as a script is to be able to reuse
// multiple helper functions available in test files

func TestGenerateAnvilStateForSimulator(t *testing.T) {
	t.Skip("Re-enable this test only when it's required to re-generate Anvil state dump")

	anvilProc, testEthUrl := initGenerateEnvForSimulator(t)
	defer stopAnvilGracefully(anvilProc)

	tc := NewTestConfig(t)
	t.Cleanup(tc.Cleanup(t))

	// deploy and config contracts on Anvil
	sethClient := test.NewSethClientWithContracts(t, L, testEthUrl, constants.TestAnvilChainID, SethConfigPath)
	_, err := test.DeployBalanceReader(sethClient)
	if err != nil {
		t.Fatalf("failed to deploy and configure BalanceReader: %v", err)
	}
}

// NOTE: this is not really a test, this is a script to re-generated Anvil EVM state dump
// once generated, state dump will contain pre-baked contracts and contract setups
// it will help with running tests
// please enable this test (comment out first line "t.Skip") and run it as standalone test only when necessary:
//
//	go test -run ^TestGenerateAnvilStateForSimulator$ -v
//
// the reason why it's created as a test rather than as a script is to be able to reuse
// multiple helper functions available in test files

func TestGenerateAnvilStateForSimulator(t *testing.T) {
	t.Skip("Re-enable this test only when it's required to re-generate Anvil state dump")

	anvilProc, testEthUrl := initGenerateEnvForSimulator(t)
	defer stopAnvilGracefully(anvilProc)

	tc := NewTestConfig(t)
	t.Cleanup(tc.Cleanup(t))

	// deploy and config contracts on Anvil
	sethClient := test.NewSethClientWithContracts(t, L, testEthUrl, constants.TestAnvilChainID, SethConfigPath)
	_, err := test.DeployBalanceReader(sethClient)
	if err != nil {
		t.Fatalf("failed to deploy and configure BalanceReader: %v", err)
	}
}
