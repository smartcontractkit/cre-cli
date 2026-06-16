## cre templates remove

Removes a template repository source

### Synopsis

Removes one or more template repository sources from your home directory (.cre/template.yaml). The ref portion is optional and ignored during matching.

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
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre templates](cre_templates.md)	 - Manages template repository sources

