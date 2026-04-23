package ethkeys

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// FormatWorkflowOwnerAddress trims whitespace, ensures a 0x prefix for hex input,
// and returns the address in EIP-55 checksummed form. Empty input (after trim)
// returns ("", nil). Non-empty input that is not a valid 20-byte hex address returns an error.
func FormatWorkflowOwnerAddress(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) < 2 || (s[0:2] != "0x" && s[0:2] != "0X") {
		s = "0x" + s
	}
	if !common.IsHexAddress(s) {
		return "", fmt.Errorf("invalid owner address %q", s)
	}
	return common.HexToAddress(s).Hex(), nil
}

func DeriveEthAddressFromPrivateKey(privateKeyHex string) (string, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf(
			"invalid private key: expected 64 hex characters (256 bits), got %d characters.\n\n"+
				"The CLI reads CRE_ETH_PRIVATE_KEY from your .env file or system environment.\n"+
				"The 0x prefix is supported and stripped automatically.\n\n"+
				"Common issues:\n"+
				"  • Pasted an Ethereum address (40 chars) instead of a private key (64 chars)\n"+
				"  • Value has extra quotes — use CRE_ETH_PRIVATE_KEY=abc123... without wrapping quotes\n"+
				"  • Key was truncated during copy-paste",
			len(privateKeyHex),
		)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("failed to cast public key to ECDSA: %w", err)
	}

	// derive the ETH address from the public key
	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return address.Hex(), nil
}
