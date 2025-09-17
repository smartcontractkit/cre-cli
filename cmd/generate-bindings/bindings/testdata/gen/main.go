package main

import (
	"github.com/smartcontractkit/dev-platform/cmd/generate-bindings/bindings"
)

func main() {
	if err := bindings.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindings",
		"",
		"./testdata/bindings.go",
	); err != nil {
		panic(err)
	}

	if err := bindings.GenerateBindings(
		"./testdata/emptybindings/EmptyContract_combined.json",
		"",
		"emptybindings",
		"",
		"./testdata/emptybindings/emptybindings.go",
	); err != nil {
		panic(err)
	}
}
