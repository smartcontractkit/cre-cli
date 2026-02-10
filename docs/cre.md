## cre

CRE CLI tool

### Synopsis

A command line tool for building, testing and managing Chainlink Runtime Environment (CRE) workflows.

```
cre [optional flags]
```

### Options

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -h, --help                  help for cre
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre account](cre_account.md)	 - Manages account
* [cre generate-bindings-evm](cre_generate-bindings-evm.md)	 - Generate bindings from contract ABI
* [cre generate-bindings-solana](cre_generate-bindings-solana.md)	 - Generate bindings from contract IDL
* [cre init](cre_init.md)	 - Initialize a new cre project (recommended starting point)
* [cre login](cre_login.md)	 - Start authentication flow
* [cre logout](cre_logout.md)	 - Revoke authentication tokens and remove local credentials
* [cre secrets](cre_secrets.md)	 - Handles secrets management
* [cre update](cre_update.md)	 - Update the cre CLI to the latest version
* [cre version](cre_version.md)	 - Print the cre version
* [cre whoami](cre_whoami.md)	 - Show your current account details
* [cre workflow](cre_workflow.md)	 - Manages workflows

