## cre generate-bindings

Generate bindings from contract ABI

### Synopsis

This command generates bindings from contract ABI files.
Supports EVM chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, IERC20.abi generates bindings in generated/ierc20/ package.

```
cre generate-bindings <chain-family> [optional flags]
```

### Examples

```
  cre generate-bindings evm
```

### Options

```
  -a, --abi string            Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)
      --go                    Generate Go bindings
  -h, --help                  help for generate-bindings
  -k, --pkg string            Base package name (each contract gets its own subdirectory) (default "bindings")
  -p, --project-root string   Path to project root directory (defaults to current directory)
      --typescript            Generate TypeScript bindings
```

### Options inherited from parent commands

```
  -e, --env string      Path to .env file which contains sensitive info (default ".env")
  -T, --target string   Use target settings from YAML config
  -v, --verbose         Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

