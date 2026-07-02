## cre execution list

List recent executions for a workflow

### Synopsis

List workflow executions from the CRE platform.

The optional argument accepts either an on-chain Workflow ID (64-char hex,
visible in 'cre workflow list') or a workflow name. When omitted, executions
across all workflows are returned.

```
cre execution list [workflow-id-or-name] [flags]
```

### Examples

```
cre execution list
  cre execution list my-workflow
  cre execution list 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856
  cre execution list my-workflow --status FAILURE
  cre execution list my-workflow --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z
  cre execution list my-workflow --limit 50 --output json
```

### Options

```
      --end string      End of time range in ISO8601 format (e.g. 2026-01-02T00:00:00Z)
  -h, --help            help for list
      --json            Output as JSON (shorthand for --output=json)
      --limit int       Maximum number of executions to return (max 100) (default 20)
      --output string   Output format: "json" prints a JSON array to stdout
      --start string    Start of time range in ISO8601 format (e.g. 2026-01-01T00:00:00Z)
      --status string   Filter by execution status (TRIGGERED, IN_PROGRESS, SUCCESS, FAILURE)
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

