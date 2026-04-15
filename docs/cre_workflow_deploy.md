## cre workflow deploy

Deploys a workflow to the Workflow Registry contract

### Synopsis

Compiles the workflow, uploads the artifacts, and registers the workflow in the Workflow Registry contract.

```
cre workflow deploy <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow deploy ./my-workflow
```

### Options

```
      --config string              Override the config file path from workflow.yaml
      --default-config             Use the config path from workflow.yaml settings (default behavior)
  -h, --help                       help for deploy
      --no-config                  Deploy without a config file
  -o, --output string              The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
  -l, --owner-label string         Label for the workflow owner (used during auto-link if owner is not already linked)
      --preview-private-registry   Deploy to the private workflow registry (unreleased feature)
      --skip-type-checks           Skip TypeScript project typecheck during compilation (passes --skip-type-checks to cre-compile)
      --unsigned                   If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
      --wasm string                Path to a pre-built WASM binary (skips compilation)
      --yes                        If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
      --non-interactive       Fail instead of prompting; requires all inputs via flags
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

