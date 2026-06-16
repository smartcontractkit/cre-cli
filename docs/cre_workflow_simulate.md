## cre workflow simulate

Simulates a workflow

### Synopsis

This command simulates a workflow.

```
cre workflow simulate <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow simulate ./my-workflow
```

### Options

```
      --broadcast                    Broadcast transactions to configured chains (default: false)
      --config string                Override the config file path from workflow.yaml
      --default-config               Use the config path from workflow.yaml settings (default behavior)
  -g, --engine-logs                  Enable non-fatal engine logging
      --evm-event-index int          EVM trigger log index (0-based) (default -1)
      --evm-receipt-timeout string   Timeout for waiting on an EVM transaction receipt (e.g. 30s, 2m) (default "1m")
      --evm-tx-hash string           EVM trigger transaction hash (0x...)
  -h, --help                         help for simulate
      --http-payload string          HTTP trigger payload as JSON string or path to JSON file
      --limits string                Production limits to enforce during simulation: 'default' for prod defaults, path to a limits JSON file (e.g. from 'cre workflow limits export'), or 'none' to disable (default "default")
      --listen                       Listen for HTTP requests or supported log triggers and run the simulator for each match
      --no-config                    Simulate without a config file
      --skip-type-checks             Skip TypeScript project typecheck during compilation (passes --skip-type-checks to cre-compile)
      --trigger-index int            Index of the trigger to run (0-based) (default -1)
      --wasm string                  Path or URL to a pre-built WASM binary (skips compilation)
```

### Options inherited from parent commands

```
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

- [cre workflow](cre_workflow.md) - Manages workflows
