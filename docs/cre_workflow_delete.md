## cre workflow delete

Deletes all versions of a workflow from the Workflow Registry

### Synopsis

Deletes all workflow versions matching the given name and owner address.

```
cre workflow delete <workflow-folder-path> [optional flags]
```

### Examples

```

		cre workflow delete ./my-workflow
		
```

### Options

```
  -h, --help       help for delete
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

