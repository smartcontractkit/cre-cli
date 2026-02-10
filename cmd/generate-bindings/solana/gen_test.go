package solana_test

import (
	"testing"

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
