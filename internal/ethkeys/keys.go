package ethkeys

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

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
