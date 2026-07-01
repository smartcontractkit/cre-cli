## cre workflow execution events

Show the node/capability event timeline for an execution

### Synopsis

Fetch and display the ordered sequence of capability events for a workflow
execution, including per-event status, method, duration, and any errors.

```
cre workflow execution events <execution-uuid> [optional flags]
```

### Examples

```
cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g
  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --capability fetch-price
  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --status FAILURE
  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json
```

### Options

```
      --capability string   Filter events to a specific capability ID
  -h, --help                help for events
      --json                Output as JSON (shorthand for --output=json)
      --output string       Output format: "json" prints a JSON array to stdout
      --status string       Filter events by status (e.g. FAILURE)
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

* [cre workflow execution](cre_workflow_execution.md)	 - Query workflow execution history

