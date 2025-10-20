## cre secrets delete

Deletes secrets from a YAML file provided as a positional argument.

```
cre secrets delete [SECRETS_FILE_PATH] [flags]
```

### Examples

```
cre secrets delete my-secrets.yaml
```

### Options

```
  -h, --help       help for delete
      --unsigned   If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Set the target settings
      --timeout duration      Timeout for secrets operations (e.g. 30m, 2h, 48h). (default 48h0m0s)
  -v, --verbose               Print DEBUG logs
```

### SEE ALSO

* [cre secrets](cre_secrets.md)	 - Handles secrets management

