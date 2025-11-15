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
  -h, --help                 help for deploy
  -o, --output string        The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
  -l, --owner-label string   Label for the workflow owner (used during auto-link if owner is not already linked)
      --unsigned             If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
      --yes                  If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

