package main

import (
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings"
)

func main() {
	if err := solana_bindings.GenerateBindings(
		"./testdata/data_storage",
		"data_storage",
		"./testdata/data_storage.json",
		"ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
	); err != nil {
		panic(err)
	}

}
