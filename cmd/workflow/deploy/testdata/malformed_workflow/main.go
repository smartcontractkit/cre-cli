package main

/*
This file contains the entry point for the WebAssembly (Wasm) executable.
To ensure the code compiles and runs correctly for Wasm (wasip1 target), we must follow these requirements:

1) **File Name**:
   The file must be named `main.go`. This is a Go convention for executables that defines where the program's entry point (`main()` function) is located.

2) **Package Name**:
   The package name must be `main`. This is essential for building an executable in Go. Go's compiler looks for a package named `main` that contains the `main()` function, which acts as the entry point of the program when the Wasm executable is run.
*/

import (
	"github.com/smartcontractkit/chainlink-common/pkg/workflows/sdk"
	"github.com/smartcontractkit/chainlink-common/pkg/workflows/wasm"
)

func BuildWorkflow(config []byte) *sdk.WorkflowSpecFactory {
	workflow := sdk.RemovedFunctionThatFailsCompilation()

	return workflow
}

func main() {
	runner := wasm.NewRunner()

	workflow := BuildWorkflow(runner.Config())
	runner.Run(workflow)
}
