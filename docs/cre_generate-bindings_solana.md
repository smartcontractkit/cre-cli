## cre generate-bindings solana

Generate bindings from contract IDL

### Synopsis

This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.

When the IDL defines events, bindings also include a `triggers.go` file with typed log-trigger helpers:

* `<Event>Filters` — optional per-field filter values (nil means wildcard for that field)
* `Encode<Event>Subkeys` — converts filter rows into `SubkeyConfig` values for the log poller
* `LogTrigger<Event>Log` — registers a typed trigger that decodes matching logs into `DecodedLog[<Event>]`

Only top-level scalar event fields are auto-filterable (pubkey, string, bytes, bool, integers, floats, and optional wrappers around those).
Nested structs, vecs, and arrays are omitted from generated filters; build `SubkeyConfig` manually for those paths.
Multiple filter rows OR values within the same field; different fields in the same registration are ANDed together.

```
cre generate-bindings solana [optional flags]
```

### Examples

```
  cre generate-bindings-solana
```

### Options

```
  -h, --help                  help for solana
  -i, --idl string            Path to IDL directory (defaults to contracts/solana/src/idl/)
  -l, --language string       Target language (go) (default "go")
  -o, --out string            Path to output directory (defaults to contracts/solana/src/generated/)
  -p, --project-root string   Path to project root directory (defaults to current directory)
```

### Options inherited from parent commands

```
      --allow-unknown-chains   Skip chain-name validation against the chain-selectors registry (for experimental chains)
  -e, --env string             Path to .env file which contains sensitive info
      --non-interactive        Fail instead of prompting; requires all inputs via flags
  -E, --public-env string      Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string          Use target settings from YAML config
  -v, --verbose                Run command in VERBOSE mode
```

### SEE ALSO

* [cre generate-bindings](cre_generate-bindings.md)	 - Generate bindings for contracts

