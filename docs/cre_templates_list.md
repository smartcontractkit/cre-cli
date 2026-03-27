## cre templates list

Lists available templates

### Synopsis

Fetches and displays all templates available from configured repository sources. These can be installed with cre init.

```
cre templates list [optional flags]
```

### Options

```
  -h, --help      help for list
      --json      Output template list as JSON
      --refresh   Bypass cache and fetch fresh data
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre templates](cre_templates.md)	 - Manages template repository sources

