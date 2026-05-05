## cre registry

Manages workflow registries

### Synopsis

The registry command lets you view and inspect the workflow registries available for your organization.

```
cre registry [optional flags]
```

### Options

```
  -h, --help   help for registry
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

* [cre](cre.md)	 - CRE CLI tool
* [cre registry list](cre_registry_list.md)	 - Lists available workflow registries for the current environment

