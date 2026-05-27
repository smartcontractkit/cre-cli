## cre registry list

Lists available workflow registries for the current environment

### Synopsis

Displays the registries configured for your organization, including type and address.

The ID shown for each registry is the value you set in workflow.yaml
under the `deployment-registry` key to target that registry, e.g.:

  <target-name>:
    user-workflow:
      workflow-name: "my-workflow"
      deployment-registry: "private"

```
cre registry list [optional flags]
```

### Examples

```
cre registry list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre registry](cre_registry.md)	 - Manages workflow registries

