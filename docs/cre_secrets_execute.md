## cre secrets execute

Executes a previously prepared MSIG bundle (.json): verifies allowlist and POSTs the exact saved request.

```
cre secrets execute [BUNDLE_PATH] [flags]
```

### Examples

```
cre secrets execute 157364...af4d5.json
```

### Options

```
  -h, --help       help for execute
      --unsigned   If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Use target settings from YAML config
      --timeout duration      Timeout for secrets operations (e.g. 30m, 2h, 48h). (default 48h0m0s)
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre secrets](cre_secrets.md)	 - Handles secrets management

