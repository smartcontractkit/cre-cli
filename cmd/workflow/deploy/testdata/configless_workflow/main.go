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
	"errors"
	"log"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/cli/cmd/testdata/fixtures/capabilities/basictrigger"
	"github.com/smartcontractkit/chainlink-common/pkg/workflows/sdk"
	"github.com/smartcontractkit/chainlink-common/pkg/workflows/wasm"
)

func BuildWorkflow(config []byte) *sdk.WorkflowSpecFactory {
	workflow := sdk.NewWorkflowSpecFactory()

	// Trigger
	triggerCfg := basictrigger.TriggerConfig{Name: "trigger", Number: 1}
	trigger := triggerCfg.New(workflow)

	// Action
	sdk.Compute1[basictrigger.TriggerOutputs, bool](
		workflow,
		"transform",
		sdk.Compute1Inputs[basictrigger.TriggerOutputs]{Arg0: trigger},
		func(sdk sdk.Runtime, outputs basictrigger.TriggerOutputs) (bool, error) {
			log.Printf("Output from the basic trigger: %v", outputs.CoolOutput)
			if outputs.CoolOutput == "cool" {
				return false, errors.New("it is cool, not good")
			}
			return true, nil
		})

	return workflow
}

func main() {
	runner := wasm.NewRunner()

	workflow := BuildWorkflow(runner.Config())
	runner.Run(workflow)
}
