## cre generate-bindings solana

Generate bindings from contract IDL

### Synopsis

This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.

Generated bindings include:

- Instruction builders and parsers
- Account and event type definitions with Borsh marshal/unmarshal
- Write report helpers (`WriteReportFrom*`) for keystone forwarder integration
- Log trigger helpers (`LogTrigger*Log`) with typed filters and `Adapt()` decoding

```
cre generate-bindings solana [optional flags]
```

### Examples

```
  cre generate-bindings solana
```

### Log trigger usage

After generating bindings, register a typed log trigger in your workflow:

```go
ds, _ := datastorage.NewDataStorage(client)

trigger, err := ds.LogTriggerAccessLoggedLog(
    chainSelector,
    "access-logged-filter",
    []datastorage.AccessLoggedFilters{
        {Caller: &expectedCaller},
    },
    nil,
)
if err != nil {
    return nil, err
}

return cre.Workflow[config.Config]{
    cre.Handler(trigger, func(cfg config.Config, rt cre.Runtime, log *bindings.DecodedLog[datastorage.AccessLogged]) (string, error) {
        return log.Data.Message, nil
    }),
}, nil
```

Filter rows use OR semantics per field (mirroring EVM log trigger topic filters). Leave a filter field nil for wildcard. Only top-level scalar event fields are filterable; nested structs, vecs, and arrays require manual `SubkeyConfig`.

For CPI-emitted events, pass `&bindings.LogTriggerOptions{CpiFilterConfig: ...}`.

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
