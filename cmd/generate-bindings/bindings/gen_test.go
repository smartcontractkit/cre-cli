package bindings_test

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings"
)

func TestGenerateBindings(t *testing.T) {
	if err := bindings.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindings",
		"",
		"./testdata/bindings.go",
	); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateBindingsOld(t *testing.T) {
	if err := bindings.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindingsold",
		"",
		"./testdata/bindingsold.go",
	); err != nil {
		t.Fatal(err)
	}
}
