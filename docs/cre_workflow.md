## cre workflow

Manages workflows

### Synopsis

The workflow command allows you to register and manage existing workflows.

```
cre workflow [optional flags]
```

### Options

```
  -h, --help   help for workflow
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

* [cre](cre.md)	 - CRE CLI tool
* [cre workflow activate](cre_workflow_activate.md)	 - Activates workflow on the Workflow Registry contract
* [cre workflow build](cre_workflow_build.md)	 - Compiles a workflow to a WASM binary
* [cre workflow custom-build](cre_workflow_custom-build.md)	 - Converts an existing workflow to a custom (self-compiled) build
* [cre workflow delete](cre_workflow_delete.md)	 - Deletes all versions of a workflow from the Workflow Registry
* [cre workflow deploy](cre_workflow_deploy.md)	 - Deploys a workflow to the Workflow Registry contract
* [cre workflow hash](cre_workflow_hash.md)	 - Computes and displays workflow hashes
* [cre workflow pause](cre_workflow_pause.md)	 - Pauses workflow on the Workflow Registry contract
* [cre workflow simulate](cre_workflow_simulate.md)	 - Simulates a workflow

