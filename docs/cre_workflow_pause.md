## cre workflow pause

Pauses workflow on the Workflow Registry contract

### Synopsis

Changes workflow status to paused on the Workflow Registry contract

```
cre workflow pause <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow pause ./my-workflow
```

### Options

```
  -h, --help             help for pause
      --profile string   Profile name for this command (overrides the active profile)
      --unsigned         If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
      --yes              If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
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

