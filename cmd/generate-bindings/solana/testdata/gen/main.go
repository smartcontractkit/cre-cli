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

	// TypeScript goldens (compared byte-for-byte by TestGenerateBindingsTS_Golden).
	if _, err := solana.GenerateBindingsTS(
		"./testdata/contracts/idl/data_storage.json",
		"data_storage",
		"./testdata/data_storage_ts",
	); err != nil {
		log.Fatal(err)
	}
	if _, err := solana.GenerateBindingsTS(
		"./testdata/contracts/idl/feature_matrix.json",
		"feature_matrix",
		"./testdata/feature_matrix_ts",
	); err != nil {
		log.Fatal(err)
	}
}
