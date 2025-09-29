## cre workflow deploy

Deploys a workflow to the Workflow Registry contract

### Synopsis

Compiles the workflow, uploads the artifacts, and registers the workflow in the Workflow Registry contract.

```
cre workflow deploy <workflow-folder-path> [flags]
```

### Examples

```

		cre workflow deploy ./my-workflow
		
```

### Options

```
  -r, --auto-start           Activate and run the workflow after registration, or pause it (default true)
  -c, --config               Should include a config file (path defined in the workflow settings file) (default: false)
  -h, --help                 help for deploy
  -k, --keep-alive           Keep previous workflows with same workflow name and owner active (default: false).
  -o, --output string        The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
  -s, --secrets-url string   URL of the encrypted secrets JSON file
  -x, --source-url string    URL of the source code in Gist
      --unsigned             If set, the command will return the raw transaction instead of sending it to the network
      --yes                  If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
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

