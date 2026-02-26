## cre init

Initialize a new cre project (recommended starting point)

### Synopsis

Initialize a new CRE project or add a workflow to an existing one.

This sets up the project structure, configuration, and starter files so you can
build, test, and deploy workflows quickly.

Templates are fetched dynamically from GitHub repositories.

```
cre init [optional flags]
```

### Options

```
  -h, --help                   help for init
  -p, --project-name string    Name for the new project
      --refresh                Bypass template cache and fetch fresh data
      --rpc-url stringArray    RPC URL for a network (format: chain-name=url, repeatable)
  -t, --template string        Name of the template to use (e.g., kv-store-go)
  -w, --workflow-name string   Name for the new workflow
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

