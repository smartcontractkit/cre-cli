## cre secrets

Handles secrets management

### Synopsis

Create, update, delete, list secrets in Vault DON.

```
cre secrets [optional flags]
```

### Options

```
  -h, --help               help for secrets
      --timeout duration   Timeout for secrets operations (e.g. 30m, 2h, 48h). (default 48h0m0s)
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
* [cre secrets create](cre_secrets_create.md)	 - Creates secrets from a YAML file.
* [cre secrets delete](cre_secrets_delete.md)	 - Deletes secrets from a YAML file provided as a positional argument.
* [cre secrets execute](cre_secrets_execute.md)	 - Executes a previously prepared MSIG bundle (.json): verifies allowlist and POSTs the exact saved request.
* [cre secrets list](cre_secrets_list.md)	 - Lists secret identifiers for the current owner address in the given namespace.
* [cre secrets update](cre_secrets_update.md)	 - Updates existing secrets from a file provided as a positional argument.

