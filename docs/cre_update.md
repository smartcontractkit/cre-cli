## cre update

Update the cre CLI to the latest version

### Synopsis

Update the cre CLI to the latest version

Release signatures are verified using the public key published by the CRE team.

On Linux, the signature is verified using GPG.
On macOS, the signature is verified using codesign.
On Windows, the signature is verified using Authenticode.

```
cre update [optional flags]
```

### Options

```
  -f, --force   Proceed with the update even if the current or latest version cannot be parsed for comparison
  -h, --help    help for update
```

### Options inherited from parent commands

```
      --allow-insecure-rpc     Allow non-localhost HTTP RPC URLs (insecure)
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

