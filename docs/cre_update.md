## cre update

Update the cre CLI to the latest version. Release signatures are verified before the binary is installed (GPG on Linux, codesign on macOS, Authenticode on Windows).

```
cre update [optional flags]
```

### Options

```
  -h, --help   help for update
```

### Options inherited from parent commands

```
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -R, --project-root string    Path to the project root
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

