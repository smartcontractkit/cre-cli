## cre init

Initialize a new workflow project or add a workflow to an existing one

### Synopsis

Initialize or extend a workflow project by setting up core files, gathering any missing details, and scaffolding the chosen template.

```
cre init [optional flags]
```

### Options

```
  -h, --help                   help for init
  -p, --project-name string    Name for the new project
  -P, --project-path string    Relative path to project root
  -t, --template-id uint32     ID of the workflow template to use
  -w, --workflow-name string   Name for the new workflow
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Set the target settings
  -v, --verbose               Print DEBUG logs
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

