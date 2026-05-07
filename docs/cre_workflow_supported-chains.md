## cre workflow supported-chains

List all supported chain names

```
cre workflow supported-chains [optional flags]
```

### Examples

```
cre workflow supported-chains
  cre workflow supported-chains --output json
```

### Options

```
  -h, --help            help for supported-chains
      --output string   Output format: "json" prints a JSON array to stdout
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

