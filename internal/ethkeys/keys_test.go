package ethkeys

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Test happy path using a freshly generated secp256k1 key.
// We derive the expected address directly from the generated public key
// and compare with what DeriveEthAddressFromPrivateKey returns.
func TestDeriveEthAddressFromPrivateKey_Valid(t *testing.T) {
	// Generate a random valid private key (secp256k1)
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Hex-encode without 0x prefix (what HexToECDSA expects)
	privHex := hex.EncodeToString(crypto.FromECDSA(priv))

	got, err := DeriveEthAddressFromPrivateKey(privHex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Independently compute the expected address from the same key
	want := crypto.PubkeyToAddress(priv.PublicKey).Hex()

	if got != want {
		t.Fatalf("address mismatch:\n  got:  %s\n  want: %s", got, want)
	}

	// Basic shape check (0x + 40 hex chars)
	if !common.IsHexAddress(got) {
		t.Fatalf("returned address is not a valid hex address: %s", got)
	}

	// Sanity check: ensure the returned address string is already in canonical checksum form
	if got != common.HexToAddress(got).Hex() {
		t.Fatalf("returned address is not in EIP-55 checksum format: %s", got)
	}
}

func TestDeriveEthAddressFromPrivateKey_InvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"nonHex", "zzzzzz"},
		{"tooShort", "deadbeef"},
		{"with0xPrefix", "0x4c0883a69102937d6231471b5dbb6204fe5129617082796fe99b4538dc2e6ea7"},
	}

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			addr, err := DeriveEthAddressFromPrivateKey(tc.in)
			if err == nil {
				t.Fatalf("expected error, got nil (addr=%q)", addr)
			}

			if !strings.Contains(strings.ToLower(err.Error()), "failed to parse private key") {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}
