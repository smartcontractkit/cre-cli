## cre templates

Manages template repository sources

### Synopsis

Manages the template repository sources that cre init uses to discover templates.

cre init ships with a default set of templates ready to use.
Use these commands only if you want to add custom or third-party template repositories.

To scaffold a new project from a template, use: cre init

```
cre templates [optional flags]
```

### Options

```
  -h, --help   help for templates
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool
* [cre templates add](cre_templates_add.md)	 - Adds a template repository source
* [cre templates list](cre_templates_list.md)	 - Lists available templates
* [cre templates remove](cre_templates_remove.md)	 - Removes a template repository source

