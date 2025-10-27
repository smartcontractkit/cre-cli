package solana_bindings_test

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings"
)

func TestGenerateBindings(t *testing.T) {
	if err := solana_bindings.GenerateBindings(
		"./testdata/data_storage",
		"data_storage",
		"./testdata/contracts/idl/data_storage.json",
		"ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
	); err != nil {
		t.Fatal(err)
	}
}
