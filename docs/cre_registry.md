## cre registry

Manages workflow registries

### Synopsis

The registry command lets you view and inspect the workflow registries available for your organization, and your usage against their quotas.

```
cre registry [optional flags]
```

### Options

```
  -h, --help   help for registry
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

* [cre](cre.md)	 - CRE CLI tool
* [cre registry list](cre_registry_list.md)	 - Lists available workflow registries for the current environment
* [cre registry quotas](cre_registry_quotas.md)	 - Shows workflow and linked-owner usage against your registry quotas

