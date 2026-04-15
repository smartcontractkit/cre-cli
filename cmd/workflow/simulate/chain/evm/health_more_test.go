package evm

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// ---------------------------------------------------------------------------
// Experimental selectors label — "experimental chain N" in error messages.
// ---------------------------------------------------------------------------

func TestHealthCheck_ExperimentalSelector_UsesExperimentalLabel(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	c := newEthClient(t, sErr.URL)
	defer c.Close()

	const expSel uint64 = 99999999
	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{expSel: c},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[experimental chain 99999999]",
	)
}

func TestHealthCheck_ExperimentalSelector_ZeroChainID_UsesExperimentalLabel(t *testing.T) {
	sZero := newChainIDServer(t, "0x0")
	defer sZero.Close()
	c := newEthClient(t, sZero.URL)
	defer c.Close()

	const expSel uint64 = 42424242
	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{expSel: c},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[experimental chain 42424242]",
		"invalid RPC response: empty or zero chain ID",
	)
}

func TestHealthCheck_UnknownSelector_FallsBackToSelectorLabel(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	c := newEthClient(t, sErr.URL)
	defer c.Close()

	const unknown uint64 = 11111
	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{unknown: c},
		nil,
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		fmt.Sprintf("[chain %d]", unknown),
	)
}

func TestHealthCheck_MixedKnownAndExperimental(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()
	cOK := newEthClient(t, sOK.URL)
	defer cOK.Close()

	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	cErr := newEthClient(t, sErr.URL)
	defer cErr.Close()

	const expSel uint64 = 99999999
	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{
			selectorSepolia: cOK,
			expSel:          cErr,
		},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[experimental chain 99999999] failed RPC health check",
	)
	// sepolia is healthy; its label must not appear.
	assert.NotContains(t, err.Error(), "[ethereum-testnet-sepolia] failed")
}

func TestHealthCheck_MultipleOK_NoError(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()
	cOK := newEthClient(t, sOK.URL)
	defer cOK.Close()

	sOK2 := newChainIDServer(t, "0x1")
	defer sOK2.Close()
	cOK2 := newEthClient(t, sOK2.URL)
	defer cOK2.Close()

	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{
			selectorSepolia: cOK,
			chainEthMainnet: cOK2,
		},
		nil,
	)
	require.NoError(t, err)
}

const chainEthMainnet uint64 = 5009297550715157269 // ethereum-mainnet

func TestHealthCheck_EmptyExperimentalMap_StillWorks(t *testing.T) {
	sOK := newChainIDServer(t, "0x1")
	defer sOK.Close()
	c := newEthClient(t, sOK.URL)
	defer c.Close()

	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{selectorSepolia: c},
		map[uint64]bool{},
	)
	require.NoError(t, err)
}

func TestHealthCheck_NilExperimentalMap_EquivalentToEmpty(t *testing.T) {
	sOK := newChainIDServer(t, "0x1")
	defer sOK.Close()
	c := newEthClient(t, sOK.URL)
	defer c.Close()

	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{selectorSepolia: c},
		nil,
	)
	require.NoError(t, err)
}

// RunRPCHealthCheck (public wrapper) — ensures ChainClient map conversion.
func TestRunRPCHealthCheck_WrapperConvertsEthClientMap(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()
	c := newEthClient(t, sOK.URL)
	defer c.Close()

	err := RunRPCHealthCheck(
		map[uint64]chain.ChainClient{selectorSepolia: c},
		map[uint64]bool{},
	)
	require.NoError(t, err)
}

func TestRunRPCHealthCheck_WrapperFailsOnNonEthClient(t *testing.T) {
	err := RunRPCHealthCheck(
		map[uint64]chain.ChainClient{1: 42}, // int masquerading as client
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for EVM family")
}

func TestRunRPCHealthCheck_EmptyReturnsSettingsError(t *testing.T) {
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check your settings")
	assert.Contains(t, err.Error(), "no RPC URLs found for supported or experimental chains")
}

func TestHealthCheck_ThreeErrors_AllLabelsInAggregated(t *testing.T) {
	sErr1 := newChainIDServer(t, fmt.Errorf("boom1"))
	defer sErr1.Close()
	cErr1 := newEthClient(t, sErr1.URL)
	defer cErr1.Close()

	sErr2 := newChainIDServer(t, fmt.Errorf("boom2"))
	defer sErr2.Close()
	cErr2 := newEthClient(t, sErr2.URL)
	defer cErr2.Close()

	err := runRPCHealthCheck(
		map[uint64]*ethclient.Client{
			selectorSepolia: cErr1,
			chainEthMainnet: cErr2,
			77777:           nil,
		},
		nil,
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[ethereum-testnet-sepolia] failed RPC health check",
		"[ethereum-mainnet] failed RPC health check",
		"[77777] nil client",
	)
}
