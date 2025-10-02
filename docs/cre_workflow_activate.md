## cre workflow activate

Activates workflow on the Workflow Registry contract

### Synopsis

Changes workflow status to active on the Workflow Registry contract

```
cre workflow activate <workflow-folder-path> [flags]
```

### Examples

```

		cre workflow activate ./my-workflow
		
```

### Options

```
  -h, --help       help for activate
      --unsigned   If set, the command will return the raw transaction instead of sending it to the network
      --yes        If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Set the target settings
  -v, --verbose               Print DEBUG logs
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

