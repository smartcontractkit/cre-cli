## cre workflow status

Show deployment health and execution summary for a workflow

### Synopsis

Show the full health picture of the workflow configured for the selected
--target in workflow.yaml: deployment status, activation state, execution
success/failure counts, and the most recent execution.

Useful for diagnosing the gap between registering a workflow and it
becoming active in the DON, or for a quick health check.

```
cre workflow status <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow status ./my-workflow --target staging
  cre workflow status ./my-workflow --target staging --output json
```

### Options

```
  -h, --help            help for status
      --json            Output as JSON (shorthand for --output=json)
      --output string   Output format: "json" prints JSON to stdout
```

### Options inherited from parent commands

```
      --allow-insecure-rpc     Allow non-localhost HTTP RPC URLs (insecure)
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

