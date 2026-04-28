## cre workflow get

Shows metadata for the workflow configured in workflow.yaml

### Synopsis

Looks up the workflow whose name is configured for the selected --target in workflow.yaml and prints its metadata from the CRE platform. By default results are filtered to the workflow's configured deployment-registry; pass --all-registries to show matches from every registry.

```
cre workflow get <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow get ./my-workflow --target staging
  cre workflow get ./my-workflow --target staging --all-registries
```

### Options

```
      --all-registries   Do not filter results by the workflow's deployment-registry
  -h, --help             help for get
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
      --non-interactive       Fail instead of prompting; requires all inputs via flags
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows

