## cre workflow logs

Show execution history for a workflow

### Synopsis

Fetches and displays recent execution history for the specified workflow from the CRE platform.

```
cre workflow logs <workflow-name> [optional flags]
```

### Examples

```
  cre workflow logs my-workflow
  cre workflow logs my-workflow --follow
  cre workflow logs my-workflow --limit 5
```

### Options

```
  -f, --follow      Keep polling for new executions
  -h, --help        help for logs
  -n, --limit int   Number of recent executions to show (default 10)
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

