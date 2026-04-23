## cre workflow list

Lists workflows deployed for your organization

### Synopsis

Lists workflows across registries in your organization. Requires authentication and user context. Deleted workflows are hidden by default.

```
cre workflow list [optional flags]
```

### Examples

```
cre workflow list
  cre workflow list --registry private
  cre workflow list --include-deleted
  cre workflow list --output /path/to/workflows.json
```

### Options

```
  -h, --help              help for list
      --include-deleted   Include workflows in DELETED status
      --output string     Write results to a .json file at the given path (relative or absolute)
      --registry string   Filter by registry ID from user context
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

