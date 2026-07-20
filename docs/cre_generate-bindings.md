## cre generate-bindings

Generate bindings for contracts

### Synopsis

The generate-bindings command allows you to generate bindings for contracts.

### Options

```
  -h, --help   help for generate-bindings
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
* [cre generate-bindings evm](cre_generate-bindings_evm.md)	 - Generate bindings from contract ABI
* [cre generate-bindings solana](cre_generate-bindings_solana.md)	 - Generate bindings from contract IDL

