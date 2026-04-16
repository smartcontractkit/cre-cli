## cre templates add

Adds a template repository source

### Synopsis

Adds one or more template repository sources to ~/.cre/template.yaml. These repositories are used by cre init to discover available templates.

```
cre templates add <owner/repo[@ref]>... [flags]
```

### Examples

```
cre templates add smartcontractkit/cre-templates@main myorg/my-templates
```

### Options

```
  -h, --help   help for add
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

* [cre templates](cre_templates.md)	 - Manages template repository sources

