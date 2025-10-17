package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type UnsignedBundle struct {
	RequestID   string          `json:"request_id"`
	Method      string          `json:"method"`
	DigestHex   string          `json:"digest_hex"`
	RequestBody json.RawMessage `json:"request_body"`
	CreatedAt   time.Time       `json:"created_at"`
}

func DeriveBundleFilename(digest [32]byte) string {
	return hex.EncodeToString(digest[:]) + ".json"
}

func SaveBundle(path string, b *UnsignedBundle) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadBundle(path string) (*UnsignedBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b UnsignedBundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	if b.RequestID == "" || b.Method == "" || b.DigestHex == "" || len(b.RequestBody) == 0 {
		return nil, fmt.Errorf("invalid bundle: missing required fields")
	}
	return &b, nil
}
