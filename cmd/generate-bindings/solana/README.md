# CRE Solana Generated Bindings

Generates CRE workflow bindings from Anchor IDL files, for **Go** and
**TypeScript**.

The Go generator wraps a forked [anchor-go](https://github.com/gagliardetto/anchor-go)
(vendored under `./anchor-go`) and adds CRE write-report extensions. The
TypeScript generator (`tsbindgen.go`) consumes the same IDLs and emits bindings
on top of `@chainlink/cre-sdk` + `@solana/codecs`.

## Usage

```bash
# auto-detects go/typescript from project files
cre generate-bindings solana

# explicit
cre generate-bindings solana --language typescript
cre generate-bindings solana --language go
```

Defaults:

| | IDL input | Output |
|---|---|---|
| Go | `contracts/solana/src/idl/*.json` | `contracts/solana/src/generated/<program>/` (one package per program) |
| TypeScript | `contracts/solana/src/idl/*.json` | `contracts/solana/ts/generated/<Program>.ts` + `<Program>_mock.ts` + `index.ts` barrel |

`--out` overrides the output directory for the selected language (rejected when
generating both languages at once — use `--language` to disambiguate).

## What gets generated (CRE-reachable surface)

Writes go through the keystone-forwarder: the on-chain entrypoint is always
`on_report`, and the payload is a bare Borsh-encoded struct. Accordingly, both
generators emit:

- per-struct write methods: `writeReportFrom<Struct>` (single) and
  `writeReportFrom<Struct>s` (Borsh `Vec`, u32-LE count + concatenated elements),
- a generic `writeReport(payload)` and `writeReportFromBorshEncodedVec(payloads)`,
- pure account/event **decoders** (discriminator-checked) — there is no
  read/simulate capability, so these only decode bytes obtained elsewhere,
- per-event **log-trigger bindings**: an `<Event>Filters` type,
  `encode<Event>Subkeys` (EQ comparers, OR across filter rows), and a typed
  `logTrigger<Event>Log(filterName, filters, opts)` method whose output adapts
  the raw log into decoded event data (Go: `bindings.DecodedLog[T]`, TS:
  `SolanaDecodedLog<T>`). `opts.cpi` targets Anchor `emit_cpi!` events. Only
  top-level scalar fields with supported subkey encodings are auto-filterable;
  nested structs, vecs, arrays, bool, u128, and i128 need a manual
  `SubkeyConfig`.
- a program mock (`new<Program>Mock`) that intercepts `writeReport` in the
  test framework.

Native Anchor instruction builders and account fetchers are **not** generated
for TypeScript: they are unreachable through the CRE capability.

The wire format mirrors the Go bindings (`cre-sdk-go` solana `bindings` package):

```
ForwarderReport = [32-byte sha256(concat account pubkeys)][u32-LE payload len][payload]
report request: EncoderName="solana", SigningAlgo="ecdsa", HashingAlgo="keccak256"
```

## TypeScript type coverage (v1)

| Anchor type | TypeScript | Codec |
|---|---|---|
| bool | boolean | `getBooleanCodec()` |
| u8…u32 / i8…i32 | number | `getU8Codec()`… |
| u64/u128 / i64/i128 | bigint | `getU64Codec()`… |
| f32 / f64 | number | `getF32Codec()` / `getF64Codec()` |
| string | string | `addCodecSizePrefix(getUtf8Codec(), getU32Codec())` |
| bytes | Uint8Array | `addCodecSizePrefix(getBytesCodec(), getU32Codec())` |
| pubkey | Address | `getAddressCodec()` |
| vec\<T\> | T[] | `getArrayCodec(inner, { size: getU32Codec() })` |
| array\<T, N\> | T[] | `getArrayCodec(inner, { size: N })` |
| option\<T\> | T \| null | `getNullableCodec(inner)` |
| defined struct | type ref | generated codec const |
| enum (scalar) | TS enum | `getEnumCodec(Enum)` (u8 tag) |

Unsupported types **fail loudly** at generation time (never silently
mis-encode): `u256`/`i256`, `COption`, data-carrying enums, tuple structs,
generics, cyclic type references.

## Tests

Golden source-compare tests live in `tsbindgen_test.go` against
`testdata/data_storage_ts/` and `testdata/feature_matrix_ts/`. Regenerate
goldens (Go + TS) with:

```bash
go generate ./cmd/generate-bindings/solana
```
