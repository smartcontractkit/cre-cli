## cre account unlink-key

Unlink a public key address from your account

### Synopsis

Unlink a previously linked public key address from your account, performing any pre-unlink cleanup.

```
cre account unlink-key [optional flags]
```

### Options

```
  -h, --help       help for unlink-key
      --unsigned   If set, the command will return the raw transaction instead of sending it to the network
      --yes        If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info (default ".env")
  -R, --project-root string   Path to the project root
  -T, --target string         Set the target settings
  -v, --verbose               Print DEBUG logs
```

### SEE ALSO

* [cre account](cre_account.md)	 - Manages account

