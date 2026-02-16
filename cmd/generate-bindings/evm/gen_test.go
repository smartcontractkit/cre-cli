package evm_test

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/evm"
)

func TestGenerateBindings(t *testing.T) {
	if err := evm.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindings",
		"",
		"./testdata/bindings.go",
	); err != nil {
		t.Fatal(err)
	}
}
