package common

import (
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/crypto"
)

// ValidatePrivateKey validates the input string is a valid Ethereum private key
func ValidatePrivateKey(key string) error {
	if !isValidPrivateKeyHex(key) {
		return fmt.Errorf("invalid private key: %s", key)
	}

	privateKeyBytes, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	_, err = crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	return nil
}

// isValidPrivateKeyHex checks if the string is a valid hexadecimal representation of a private key
func isValidPrivateKeyHex(key string) bool {
	match, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", key)
	return match
}
