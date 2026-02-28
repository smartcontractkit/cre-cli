## cre workflow limits export

Export default simulation limits as JSON

### Synopsis

Exports the default production simulation limits as JSON.

The output can be redirected to a file and customized for use with
the --limits flag of the simulate command.

Example:
  cre workflow limits export > my-limits.json
  cre workflow simulate ./my-workflow --limits ./my-limits.json

```
cre workflow limits export [optional flags]
```

### Options

```
  -h, --help   help for export
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow limits](cre_workflow_limits.md)	 - Manage simulation limits

