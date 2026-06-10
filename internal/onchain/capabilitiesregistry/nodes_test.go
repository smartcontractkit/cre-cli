package capabilitiesregistry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
)

func TestOCRSignerAddresses(t *testing.T) {
	t.Parallel()

	var signerA, signerB [32]byte
	copy(signerA[12:], common.Hex2Bytes("1111111111111111111111111111111111111111"))
	copy(signerB[12:], common.Hex2Bytes("2222222222222222222222222222222222222222"))

	signers, err := OCRSignerAddresses([]capreg.INodeInfoProviderNodeInfo{
		{P2pId: [32]byte{1}, Signer: signerA},
		{P2pId: [32]byte{2}, Signer: signerB},
	})
	require.NoError(t, err)
	require.Equal(t, []common.Address{
		common.BytesToAddress(signerA[:20]),
		common.BytesToAddress(signerB[:20]),
	}, signers)
}

func TestMinOCRSignatures(t *testing.T) {
	t.Parallel()
	require.Equal(t, 3, MinOCRSignatures(2))
}
