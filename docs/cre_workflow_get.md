## cre workflow get

Show deployment health and recent execution for the workflow in workflow.yaml

### Synopsis

Looks up the workflow whose name is configured for the selected --target in workflow.yaml and prints deployment health and the most recent execution from the CRE platform. By default resolution is scoped to the workflow's configured deployment-registry; pass --all-registries to resolve across every registry.

```
cre workflow get <workflow-folder-path> [optional flags]
```

### Examples

```
cre workflow get ./my-workflow --target staging
  cre workflow get ./my-workflow --target staging --all-registries
  cre workflow get ./my-workflow --target staging --output json
```

### Options

```
      --all-registries   Resolve the workflow across every registry instead of the configured deployment-registry
  -h, --help             help for get
      --json             Output as JSON (shorthand for --output=json)
      --output string    Output format: "json" prints JSON to stdout
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

