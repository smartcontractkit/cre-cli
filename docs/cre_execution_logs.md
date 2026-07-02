## cre execution logs

Show logs emitted during a workflow execution

### Synopsis

Fetch and display all log lines emitted during a workflow execution.
Use --node to filter to a specific capability node (client-side filter).

```
cre execution logs <execution-uuid> [optional flags]
```

### Examples

```
cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g
  cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --node ProcessData
  cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json
```

### Options

```
  -h, --help            help for logs
      --json            Output as JSON (shorthand for --output=json)
      --node string     Filter logs to a specific node/capability ID (case-insensitive)
      --output string   Output format: "json" prints a JSON array to stdout
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

