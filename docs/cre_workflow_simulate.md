## cre workflow simulate

Simulates a workflow

### Synopsis

This command simulates a workflow.

```
cre workflow simulate ./path/to/workflow/main.go [flags]
```

### Options

```
      --broadcast             Broadcast transactions to the EVM (default: false)
  -c, --config string         Path to the config file
  -g, --engine-logs           Enable non-fatal engine logging
      --evm-chain string      EVM chain to use for EVM triggers in non-interactive mode (eth-sepolia|eth-mainnet)
      --evm-event-index int   EVM trigger log index (0-based) (default -1)
      --evm-tx-hash string    EVM trigger transaction hash (0x...)
  -h, --help                  help for simulate
      --http-payload string   HTTP trigger payload as JSON string or path to JSON file (with or without @ prefix)
      --non-interactive       Run without prompts; requires --trigger-index and inputs for the selected trigger type
  -s, --secrets string        Path to the secrets file
      --trigger-index int     Index of the trigger to run (0-based) (default -1)
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

