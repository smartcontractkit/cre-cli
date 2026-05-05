## cre workflow hash

Computes and displays workflow hashes

### Synopsis

Computes the binary hash, config hash, and workflow hash for a workflow. The workflow hash uses the same algorithm as the on-chain workflow ID.

```
cre workflow hash <workflow-folder-path> [optional flags]
```

### Examples

```
  cre workflow hash ./my-workflow
  cre workflow hash ./my-workflow --public_key 0x1234...abcd
```

### Options

```
      --config string       Override the config file path from workflow.yaml
      --default-config      Use the config path from workflow.yaml settings (default behavior)
  -h, --help                help for hash
      --no-config           Hash without a config file
      --public_key string   Owner address to use for computing the workflow hash. Required when CRE_ETH_PRIVATE_KEY is not set and no workflow-owner-address is configured. Defaults to the address derived from CRE_ETH_PRIVATE_KEY or the workflow-owner-address in project settings.
      --skip-type-checks    Skip TypeScript project typecheck during compilation (passes --skip-type-checks to cre-compile)
      --wasm string         Path or URL to a pre-built WASM binary (skips compilation)
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

