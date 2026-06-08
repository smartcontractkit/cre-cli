package main

import (
	"log"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana"
)

func main() {
	if err := solana.GenerateBindings(
		"./testdata/contracts/idl/data_storage.json",
		"data_storage",
		"./testdata/data_storage",
	); err != nil {
		log.Fatal(err)
	}
}
