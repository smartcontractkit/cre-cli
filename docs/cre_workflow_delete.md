## cre workflow delete

Deletes all versions of a workflow from the Workflow Registry

### Synopsis

Deletes all workflow versions matching the given name and owner address.

```
cre workflow delete [flags]
```

### Options

```
  -h, --help                help for delete
  -y, --skip-confirmation   Force delete workflow without confirmation
      --unsigned            If set, the command will return the raw transaction instead of sending it to the network [EXPERIMENTAL]
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

