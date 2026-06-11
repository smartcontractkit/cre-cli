package capabilitiesregistry

import (
	"github.com/ethereum/go-ethereum/common"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
)

// OCRSignerAddresses returns the OCR signer addresses for CapabilitiesRegistry member nodes.
// Signer is a bytes32 on-chain; the address is stored in the first 20 bytes (see chainlink launcher).
func OCRSignerAddresses(nodes []capreg.INodeInfoProviderNodeInfo) []common.Address {
	signers := make([]common.Address, 0, len(nodes))
	for _, node := range nodes {
		signers = append(signers, common.BytesToAddress(node.Signer[:20]))
	}
	return signers
}

// MinOCRSignatures returns the minimum valid OCR signature count (F+1) for a DON.
func MinOCRSignatures(f uint8) int {
	return int(f) + 1
}
