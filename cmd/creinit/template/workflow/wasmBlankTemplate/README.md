# Self-compiled WASM Workflow Template

This template provides a blank workflow template for self-compiled WASM workflows. It includes the necessary files for a workflow, excluding workflow code.

## Structure

- `Makefile`: Contains a TODO on the `build` target where you should add your build logic
- `workflow.yaml`: Workflow settings file with the wasm directory configured
- `config.staging.json` and `config.production.json`: Configuration files for different environments
- `secrets.yaml`: Secrets file (if needed)

## Steps to use

1. **Add your build logic**: Edit the `Makefile` and implement the `build` target. This should compile your workflow to `wasm/workflow.wasm`.

2. **Build your workflow**: Run `make build` from the workflow directory.

3. **Simulate the workflow**: From the project root, run:
   ```bash
   cre workflow simulate <workflow-name> --target=staging-settings
   ```

## Example Makefile build target

```makefile
# For Go workflows:
export GOOS := wasip1
export GOARCH := wasm
export CGO_ENABLED := 0

build:
	go build -o wasm/workflow.wasm .
```
