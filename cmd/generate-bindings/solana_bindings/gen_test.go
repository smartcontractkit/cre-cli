package solana_bindings_test

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings"
)

func TestGenerateBindings(t *testing.T) {
	if err := solana_bindings.GenerateBindings(
		"./testdata/contracts/idl/data_storage.json",
		"data_storage",
		"./testdata/data_storage",
	); err != nil {
		t.Fatal(err)
	}
}
