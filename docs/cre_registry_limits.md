## cre registry limits

Shows workflow usage and limits for the current registries

### Synopsis

Displays how many workflows each registry is running against its limit.

For the private registry, limits are per DON family. For an on-chain registry, the linked-owner
quota is shown first, followed by the on-chain workflow count and limit for each linked owner.

Usage is fetched live on every run; it is not read from the cached tenant context.

```
cre registry limits [optional flags]
```

### Examples

```
cre registry limits
```

### Options

```
  -h, --help   help for limits
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

