## cre workflow limits export

Export default simulation limits as JSON

### Synopsis

Exports the default production simulation limits as JSON.
The output can be redirected to a file and customized.

```
cre workflow limits export [optional flags]
```

### Examples

```
cre workflow limits export > my-limits.json
```

### Options

```
  -h, --help   help for export
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

* [cre workflow limits](cre_workflow_limits.md)	 - Manage simulation limits

