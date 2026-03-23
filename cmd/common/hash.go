package common

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashBytes computes the SHA-256 hash of data and returns it as a hex string.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
