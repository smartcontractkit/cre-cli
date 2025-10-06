## cre generate-bindings

Generate bindings from contract ABI

### Synopsis

This command generates bindings from contract ABI files.
Supports EVM chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, IERC20.abi generates bindings in generated/ierc20/ package.

```
cre generate-bindings <chain-family> [flags]
```

### Examples

```
  cre generate-bindings evm
```

### Options

```
  -a, --abi string            Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)
  -h, --help                  help for generate-bindings
  -l, --language string       Target language (go) (default "go")
  -k, --pkg string            Base package name (each contract gets its own subdirectory) (default "bindings")
  -p, --project-root string   Path to project root directory (defaults to current directory)
```

### Options inherited from parent commands

```
  -e, --env string      Path to .env file which contains sensitive info (default ".env")
  -T, --target string   Set the target settings
  -v, --verbose         Print DEBUG logs
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

