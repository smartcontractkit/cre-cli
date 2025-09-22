## cre workflow deploy

Deploys a workflow to the Workflow Registry contract

### Synopsis

Compiles the workflow, uploads the artifacts, and registers the workflow in the Workflow Registry contract.

```
cre workflow deploy [flags]
```

### Options

```
  -r, --auto-start           Activate and run the workflow after registration, or pause it (default true)
  -c, --config string        Path to the config file
  -h, --help                 help for deploy
  -k, --keep-alive           Keep previous workflows with same workflow name and owner active (default: false).
  -o, --output string        The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
  -s, --secrets-url string   URL of the encrypted secrets JSON file
  -x, --source-url string    URL of the source code in Gist
      --unsigned             If set, the command will return the raw transaction instead of sending it to the network
```

### Options inherited from parent commands

```
  -e, --env string                      Path to .env file which contains sensitive info (default ".env")
  -T, --target string                   Set the target settings
  -v, --verbose                         Print DEBUG logs
  -S, --workflow-settings-file string   Path to CLI workflow settings file (default "workflow.yaml")
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

