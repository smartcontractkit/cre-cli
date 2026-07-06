package solana_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana"
)

func TestGenerateBindings(t *testing.T) {
	if err := solana.GenerateBindings(
		"./testdata/contracts/idl/data_storage.json",
		"data_storage",
		"./testdata/data_storage",
	); err != nil {
		t.Fatal(err)
	}
}

// TestGenerateBindings_MissingAddress verifies that an IDL without an address
// field succeeds (previously it returned an error). program_id.go must not be
// generated since there is no address to embed.
func TestGenerateBindings_MissingAddress(t *testing.T) {
	idl := `{
  "metadata": {"name": "no_addr", "version": "0.1.0", "spec": "0.1.0"},
  "instructions": [
    {"name": "on_report", "discriminator": [214,173,18,221,173,148,151,208], "accounts": [], "args": []}
  ],
  "accounts": [],
  "events": [],
  "errors": [],
  "types": []
}`
	idlPath := filepath.Join(t.TempDir(), "no_addr.json")
	require.NoError(t, os.WriteFile(idlPath, []byte(idl), 0o600))

	outDir := t.TempDir()
	err := solana.GenerateBindings(idlPath, "no_addr", outDir)
	require.NoError(t, err)

	// program_id.go must not be generated when there is no address in the IDL.
	_, statErr := os.Stat(filepath.Join(outDir, "program_id.go"))
	assert.True(t, os.IsNotExist(statErr), "program_id.go should not be generated when IDL has no address")
}
