## cre

CRE CLI tool

### Synopsis

A command line tool for building, testing and managing Chainlink Runtime Environment (CRE) workflows.

```
cre [optional flags]
```

### Options

```
  -e, --env string            Path to .env file which contains sensitive info
  -h, --help                  help for cre
      --non-interactive       Fail instead of prompting; requires all inputs via flags
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre account](cre_account.md)	 - Manage account and request deploy access
* [cre generate-bindings](cre_generate-bindings.md)	 - Generate bindings from contract ABI
* [cre init](cre_init.md)	 - Initialize a new cre project (recommended starting point)
* [cre login](cre_login.md)	 - Start authentication flow
* [cre logout](cre_logout.md)	 - Revoke authentication tokens and remove local credentials
* [cre registry](cre_registry.md)	 - Manages workflow registries
* [cre secrets](cre_secrets.md)	 - Handles secrets management
* [cre templates](cre_templates.md)	 - Manages template repository sources
* [cre update](cre_update.md)	 - Update the cre CLI to the latest version
* [cre version](cre_version.md)	 - Print the cre version
* [cre whoami](cre_whoami.md)	 - Show your current account details
* [cre workflow](cre_workflow.md)	 - Manages workflows

