## cre workflow build

Compiles a workflow to a WASM binary

### Synopsis

Compiles the workflow to WASM and writes the raw binary to a file. Does not upload, register, or simulate.

```
cre workflow build <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow build ./my-workflow
```

### Options

```
  -h, --help               help for build
  -o, --output string      Output file path for the compiled WASM binary (default: <workflow-folder>/binary.wasm)
      --skip-type-checks   Skip TypeScript project typecheck during compilation (passes --skip-type-checks to cre-compile)
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

