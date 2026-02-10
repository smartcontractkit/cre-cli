## cre generate-bindings-solana

Generate bindings from contract IDL

### Synopsis

This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.

```
cre generate-bindings-solana [optional flags]
```

### Examples

```
  cre generate-bindings-solana
```

### Options

```
  -a, --abi string            Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)
  -h, --help                  help for generate-bindings-solana
  -l, --language string       Target language (go) (default "go")
  -k, --pkg string            Base package name (each contract gets its own subdirectory) (default "bindings")
  -p, --project-root string   Path to project root directory (defaults to current directory)
```

### Options inherited from parent commands

```
  -e, --env string      Path to .env file which contains sensitive info (default ".env")
  -T, --target string   Use target settings from YAML config
  -v, --verbose         Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

