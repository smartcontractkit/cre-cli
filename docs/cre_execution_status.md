## cre execution status

Show detailed status of a single execution

### Synopsis

Fetch and display the full status of a workflow execution, including
top-level errors when the execution has failed.

```
cre execution status <execution-uuid> [optional flags]
```

### Examples

```
cre execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g
  cre execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json
```

### Options

```
  -h, --help            help for status
      --json            Output as JSON (shorthand for --output=json)
      --output string   Output format: "json" prints JSON to stdout
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

* [cre execution](cre_execution.md)	 - Query workflow execution history

