## cre account unlink-key

Unlink a public key address from your account

### Synopsis

Unlink a previously linked public key address from your account, performing any pre-unlink cleanup.

```
cre account unlink-key [flags]
```

### Options

```
  -h, --help                help for unlink-key
      --non-interactive     If set, the command will skip all interactive prompts and fail if any required information is missing
  -y, --skip-confirmation   Force unlink without confirmation
      --unsigned            If set, the command will return the raw transaction instead of sending it to the network
```

### Options inherited from parent commands

```
  -e, --env string                      Path to .env file which contains sensitive info (default ".env")
  -T, --target string                   Set the target settings
  -v, --verbose                         Print DEBUG logs
  -S, --workflow-settings-file string   Path to CLI workflow settings file (default "workflow.yaml")
```

### SEE ALSO

* [cre account](cre_account.md)	 - Manages account

