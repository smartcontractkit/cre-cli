## cre workflow validate

Validate workflow source against its latest deployment

### Synopsis

Verifies that a workflow's source code reproduces the binary and config deployed on-chain, and marks the deployment verified when it matches.

```
cre workflow validate <workflow-folder-path> [optional flags]
```

### Examples

```
  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/releases/tag/v1.2.0
  cre workflow validate ./my-workflow --source-code ./shared-source/acme-workflow
  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/tree/main --workflow-id 0x00986c...78038b
  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/releases/tag/v1.2.0 --no-publish
```

### Options

```
  -h, --help                   help for validate
      --no-publish             Validate only; do not update the published code URL
      --source-code string     Path or URL to the workflow source (public URL, or local path to privately shared code)
      --workflow-id string     Workflow ID of the deployment to validate against (defaults to the latest deployment)
      --yes                    Skip the confirmation prompt before publishing the code URL
```

### Options inherited from parent commands

```
      --allow-insecure-rpc     Allow non-localhost HTTP RPC URLs (insecure)
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre workflow](cre_workflow.md)	 - Manages workflows
