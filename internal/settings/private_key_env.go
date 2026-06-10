package settings

import (
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
)

// privateKeyEnvPlaceholders are default template values from cre init that should
// be treated as unset rather than validated as hex private keys.
var privateKeyEnvPlaceholders = map[string]struct{}{
	DefaultEthPrivateKeyEnvPlaceholder: {},
}

// ResolveEthPrivateKeyFromEnv interprets CRE_ETH_PRIVATE_KEY for settings load.
// Empty and cre-init placeholder values return ("", nil). Any other non-empty
// value is validated; malformed keys return an error with guidance. Valid keys
// return the normalized hex string without a 0x prefix.
func ResolveEthPrivateKeyFromEnv(raw string) (EthPrivateKeyHex, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if isPrivateKeyEnvPlaceholder(raw) {
		return "", nil
	}

	norm := NormalizeHexKey(raw)
	if _, err := ethkeys.DeriveEthAddressFromPrivateKey(norm); err != nil {
		return "", err
	}
	return EthPrivateKeyHex(norm), nil
}

// IsUsablePrivateKeyHex reports whether raw looks like an intentional private key
// value (non-empty, non-placeholder, valid 64-character hex).
func IsUsablePrivateKeyHex(raw string) bool {
	key, err := ResolveEthPrivateKeyFromEnv(raw)
	return err == nil && key.IsSet()
}

func isPrivateKeyEnvPlaceholder(s string) bool {
	_, ok := privateKeyEnvPlaceholders[strings.TrimSpace(s)]
	return ok
}
