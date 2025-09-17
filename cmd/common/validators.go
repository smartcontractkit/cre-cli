package common

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
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

// Validates if the chain ID can be translated to chain selector, returns the chain selector once it's found
func GetChainSelectorBasedOnChainId(l *zerolog.Logger, chainIdStr string) (uint64, error) {
	l.Info().
		Str("Chain ID", chainIdStr).
		Msg("Validating chain ID, trying to find chain selector")

	if chainIdStr == "" {
		return 0, errors.New("chain id can't be empty")
	}

	chainId, err := strconv.ParseUint(chainIdStr, 10, 64)
	if err != nil {
		return 0, errors.New("chain id is not uin64")
	}
	chainSelector, err := chain_selectors.SelectorFromChainId(chainId)
	if err != nil {
		return 0, errors.New("chain selector match for this chain id not found")
	}

	isEvm, err := chain_selectors.IsEvm(chainSelector)
	if err != nil {
		return 0, errors.New("chain selector is not valid")
	}

	if !isEvm {
		return 0, errors.New("this tool currently only supports chains from the EVM family")
	}

	l.Info().
		Str("Chain ID", chainIdStr).
		Uint64("Chain Selector", chainSelector).
		Msg("Chain ID and chain selector mapping found")
	return chainSelector, nil
}

// isValidPrivateKeyHex checks if the string is a valid hexadecimal representation of a private key
func isValidPrivateKeyHex(key string) bool {
	match, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", key)
	return match
}
