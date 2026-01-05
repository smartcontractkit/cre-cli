## cre workflow generate-id

Display the workflow ID

```
cre workflow generate-id <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow generate-id ./my-workflow
```

### Options

```
  -h, --help            help for generate-id
  -o, --output string   The output file for the compiled WASM binary encoded in base64 (default "./binary.wasm.br.b64")
      --owner string    Workflow owner address
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

