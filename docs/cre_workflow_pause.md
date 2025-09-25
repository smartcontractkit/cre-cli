## cre workflow pause

Pauses workflow on the Workflow Registry contract

### Synopsis

Changes workflow status to paused on the Workflow Registry contract

```
cre workflow pause [flags]
```

### Options

```
  -h, --help       help for pause
      --unsigned   If set, the command will return the raw transaction instead of sending it to the network
      --yes        If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
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

