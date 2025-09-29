## cre account link-key

Link a public key address to your account

### Synopsis

Link a public key address to your account for workflow operations.

```
cre account link-key [flags]
```

### Options

```
  -h, --help                 help for link-key
  -l, --owner-label string   Label for the workflow owner
      --unsigned             If set, the command will return the raw transaction instead of sending it to the network
      --yes                  If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive
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

