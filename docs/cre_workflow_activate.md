## cre workflow activate

Activates workflow on the Workflow Registry contract

### Synopsis

Changes workflow status to active on the Workflow Registry contract.

The argument can be a workflow folder path (project-based flow), a workflow
name, or an on-chain workflow ID. When the argument is a name or ID — or is
omitted for an interactive picker — no project folder is needed; the workflow
is resolved from the CRE platform. On-chain workflows additionally require
--rpc-url and CRE_ETH_PRIVATE_KEY in that mode. --target and --project-root
are ignored in this mode.

```
cre workflow activate [workflow-name-or-id | workflow-folder-path] [flags]
```

### Examples

```
cre workflow activate ./my-workflow
  cre workflow activate my-workflow
  cre workflow activate 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856
  cre workflow activate my-workflow --rpc-url https://... --yes
  cre workflow activate   # interactive picker
```

### Options

```
  -h, --help              help for activate
  -o, --owner string      Scope workflow name resolution to this owner address
      --registry string   Scope workflow resolution to a registry ID (see 'cre workflow list')
      --rpc-url string    RPC URL of the workflow registry chain (required for on-chain workflows when no project folder is used)
      --unsigned          If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
      --yes               If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
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

