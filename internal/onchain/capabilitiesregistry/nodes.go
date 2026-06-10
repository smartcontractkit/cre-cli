package capabilitiesregistry

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
)

// OCRSignerAddresses returns the OCR signer addresses for CapabilitiesRegistry member nodes.
func OCRSignerAddresses(nodes []capreg.INodeInfoProviderNodeInfo) ([]common.Address, error) {
	signers := make([]common.Address, 0, len(nodes))
	for _, node := range nodes {
		if len(node.Signer) < 20 {
			return nil, fmt.Errorf("node signer address too short for p2p id %x", node.P2pId)
		}
		signers = append(signers, common.BytesToAddress(node.Signer[:20]))
	}
	return signers, nil
}

// MinOCRSignatures returns the minimum valid OCR signature count (F+1) for a DON.
func MinOCRSignatures(f uint8) int {
	return int(f) + 1
}
