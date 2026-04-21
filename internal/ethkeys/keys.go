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
		return "", fmt.Errorf("failed to parse private key. Please check CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
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
