## cre registry quotas

Shows workflow and linked-owner usage against your registry quotas

### Synopsis

Shows, for each workflow registry available to your organization, how many
workflows are active against the limit of each DON family, and how many owner
addresses are linked against the linked-owner limit.

Pass a registry ID from `cre registry list` to show a single registry.

Pending link requests count against the linked-owner limit until they expire.

```
cre registry quotas [registry-id] [flags]
```

### Examples

```
  cre registry quotas
  cre registry quotas private
  cre registry quotas --json
```

### Options

```
  -h, --help            help for quotas
      --json            Output as JSON (shorthand for --output=json)
      --output string   Output format: "json" prints the quotas as JSON to stdout
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

* [cre registry](cre_registry.md)	 - Manages workflow registries

