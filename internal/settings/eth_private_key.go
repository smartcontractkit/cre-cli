package settings

import "fmt"

const (
	// DefaultEthPrivateKeyEnvPlaceholder is written into generated .env files by cre init
	// and treated as unset when resolving CRE_ETH_PRIVATE_KEY.
	DefaultEthPrivateKeyEnvPlaceholder = "your-eth-private-key"
)

// EthPrivateKeyHex holds a normalized ECDSA private key hex string (no 0x prefix).
// String and GoString redact the value so accidental logging, fmt, or telemetry
// serialization does not leak the key (e.g. in CI output).
type EthPrivateKeyHex string

// Hex returns the raw private key hex for cryptographic use.
func (k EthPrivateKeyHex) Hex() string {
	return string(k)
}

// IsSet reports whether a usable private key is present.
func (k EthPrivateKeyHex) IsSet() bool {
	return k != ""
}

func (k EthPrivateKeyHex) String() string {
	if k == "" {
		return ""
	}
	return "[REDACTED]"
}

func (k EthPrivateKeyHex) GoString() string {
	return fmt.Sprintf("settings.EthPrivateKeyHex(%s)", k.String())
}
