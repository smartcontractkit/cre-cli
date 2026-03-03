## cre templates remove

Removes a template repository source

### Synopsis

Removes one or more template repository sources from ~/.cre/template.yaml. The ref portion is optional and ignored during matching.

```
cre templates remove <owner/repo>... [optional flags]
```

### Examples

```
cre templates remove smartcontractkit/cre-templates myorg/my-templates
```

### Options

```
  -h, --help   help for remove
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre templates](cre_templates.md)	 - Manages template repository sources

