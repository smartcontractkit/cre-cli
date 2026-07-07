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

The Solana CRE capability is **write-only** through the keystone-forwarder: the
on-chain entrypoint is always `on_report`, and the payload is a bare
Borsh-encoded struct. Accordingly, both generators emit:

- per-struct write methods: `writeReportFrom<Struct>` (single) and
  `writeReportFrom<Struct>s` (Borsh `Vec`, u32-LE count + concatenated elements),
- a generic `writeReport(payload)` and `writeReportFromBorshEncodedVec(payloads)`,
- pure account/event **decoders** (discriminator-checked) — there is no
  read/simulate capability, so these only decode bytes obtained elsewhere,
- a program mock (`new<Program>Mock`) that intercepts `writeReport` in the
  test framework.

Native Anchor instruction builders and account fetchers are **not** generated
for TypeScript: they are unreachable through the write-only capability.

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

## Simulation (`cre workflow simulate`)

The simulator never writes through the real keystone forwarder (it cannot
produce DON signatures). It writes through a per-chain **mock forwarder**
(`cre workflow supported-chains`; wired via `FakeSolanaChain` from
`chainlink-solana/contracts/capabilities/fakes`).

Generated bindings need **no simulation-specific config**: keep the real
forwarder state/authority in `remainingAccounts[0..1]` as for production. The
simulator strips those two accounts and rewrites the report's embedded
account hash for the mock forwarder before sending (the mock forwarder skips
DON-signature verification, so the rewrite is safe).

The one thing the simulator cannot do is satisfy the **receiver program's own
caller validation**. If your receiver verifies its trusted forwarder (state
owner / authority PDA / stored forwarder program id — the keystone pattern),
initialize its trust anchor once against the mock forwarder program id on
devnet and point the simulation target's config at that state. This is the
Solana analog of deploying an EVM receiver with the documented (mock)
forwarder address as a constructor argument. Receivers without caller
validation simulate with the production config as-is.

## Tests

Golden source-compare tests live in `tsbindgen_test.go` against
`testdata/data_storage_ts/` and `testdata/feature_matrix_ts/`. Regenerate
goldens (Go + TS) with:

```bash
go generate ./cmd/generate-bindings/solana
```
