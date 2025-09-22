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
      --non-interactive      If set, the command will skip all interactive prompts and fail if any required information is missing
  -l, --owner-label string   Label for the workflow owner
      --unsigned             If set, the command will return the raw transaction instead of sending it to the network
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

