## cre logout

Revoke authentication tokens and remove local credentials

### Synopsis

Invalidates the current authentication tokens and deletes stored credentials.

```
cre logout [optional flags]
```

### Options

```
  -h, --help   help for logout
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
      --non-interactive       Fail instead of prompting; requires all inputs via flags
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

