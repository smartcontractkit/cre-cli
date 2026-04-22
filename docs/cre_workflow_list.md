## cre workflow list

Lists workflows deployed for your organization

### Synopsis

Lists workflows across registries using the platform API. Requires authentication and user context (context.yaml). Does not use a workflow folder or --target. Deleted workflows are hidden by default.

```
cre workflow list [optional flags]
```

### Examples

```
cre workflow list
  cre workflow list --registry private
  cre workflow list --include-deleted
```

### Options

```
  -h, --help              help for list
      --include-deleted   Include workflows in DELETED status
      --registry string   Filter by registry ID from context.yaml (e.g. private)
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

