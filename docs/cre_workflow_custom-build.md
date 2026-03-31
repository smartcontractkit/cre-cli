## cre workflow custom-build

Converts an existing workflow to a custom (self-compiled) build

### Synopsis

Converts a Go or TypeScript workflow to use a custom build via Makefile, producing wasm/workflow.wasm. The workflow-path in workflow.yaml is updated to ./wasm/workflow.wasm. This cannot be undone.

```
cre workflow custom-build <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow custom-build ./my-workflow
```

### Options

```
  -f, --force             Skip confirmation prompt and convert immediately
  -h, --help              help for custom-build
      --non-interactive   Fail instead of prompting; requires all inputs via flags
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

