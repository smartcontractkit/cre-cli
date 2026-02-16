package main

import (
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/evm"
)

func main() {
	if err := evm.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindings",
		"",
		"./testdata/bindings.go",
	); err != nil {
		panic(err)
	}

	if err := evm.GenerateBindings(
		"./testdata/emptybindings/EmptyContract_combined.json",
		"",
		"emptybindings",
		"",
		"./testdata/emptybindings/emptybindings.go",
	); err != nil {
		panic(err)
	}
}
