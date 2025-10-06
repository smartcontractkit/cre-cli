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
  -r, --auto-start      Activate and run the workflow after registration, or pause it (default true)
  -h, --help            help for deploy
  -o, --output string   The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
      --unsigned        If set, the command will return the raw transaction instead of sending it to the network
      --yes             If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Set the target settings
  -v, --verbose               Print DEBUG logs
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

