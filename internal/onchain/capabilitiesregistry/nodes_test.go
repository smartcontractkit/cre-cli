package capabilitiesregistry

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
)

func TestOCRSignerAddresses(t *testing.T) {
	t.Parallel()

	addrA := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addrB := common.HexToAddress("0x2222222222222222222222222222222222222222")

	var signerA, signerB [32]byte
	copy(signerA[:20], addrA.Bytes())
	copy(signerB[:20], addrB.Bytes())

	signers := OCRSignerAddresses([]capreg.INodeInfoProviderNodeInfo{
		{P2pId: [32]byte{1}, Signer: signerA},
		{P2pId: [32]byte{2}, Signer: signerB},
	})
	require.Equal(t, []common.Address{addrA, addrB}, signers)
}

func TestMinOCRSignatures(t *testing.T) {
	t.Parallel()
	require.Equal(t, 3, MinOCRSignatures(2))
}
