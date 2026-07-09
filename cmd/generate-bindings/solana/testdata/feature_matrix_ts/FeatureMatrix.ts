// Code generated — DO NOT EDIT.
import {
  addCodecSizePrefix,
  getArrayCodec,
  getBooleanCodec,
  getBytesCodec,
  getEnumCodec,
  getF32Codec,
  getF64Codec,
  getI128Codec,
  getI16Codec,
  getI32Codec,
  getI64Codec,
  getI8Codec,
  getNullableCodec,
  getStructCodec,
  getU128Codec,
  getU16Codec,
  getU32Codec,
  getU64Codec,
  getU8Codec,
  getUtf8Codec,
} from '@solana/codecs'
import { getAddressCodec, type Address } from '@solana/addresses'
import {
  bytesToHex,
  calculateAccountsHash,
  encodeBorshVecU32,
  encodeForwarderReport,
  prepareSolanaReportRequest,
  type Runtime,
  type SolanaAccountMeta,
  SolanaClient,
  solanaAccountMetasToJson,
  solanaAddressToBytes,
  type SolanaComputeConfig,
} from '@chainlink/cre-sdk'

export const FEATURE_MATRIX_PROGRAM_ID = 'ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL'

export const FEATURE_MATRIX_IDL = {"address":"ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL","metadata":{"name":"feature_matrix","version":"0.1.0","spec":"0.1.0","description":"Fixture exercising the v1 TypeScript type-coverage matrix"},"instructions":[{"name":"on_report","discriminator":[214,173,18,221,173,148,151,208],"accounts":[],"args":[{"name":"_metadata","type":"bytes"},{"name":"payload","type":"bytes"}]}],"accounts":[],"events":[],"errors":[],"types":[{"name":"Color","type":{"kind":"enum","variants":[{"name":"Red"},{"name":"Green"},{"name":"Blue"}]}},{"name":"Inner","type":{"kind":"struct","fields":[{"name":"id","type":"u32"},{"name":"label","type":"string"}]}},{"name":"Everything","type":{"kind":"struct","fields":[{"name":"flag","type":"bool"},{"name":"tiny","type":"u8"},{"name":"tiny_signed","type":"i8"},{"name":"small","type":"u16"},{"name":"small_signed","type":"i16"},{"name":"medium","type":"u32"},{"name":"medium_signed","type":"i32"},{"name":"big","type":"u64"},{"name":"big_signed","type":"i64"},{"name":"huge","type":"u128"},{"name":"huge_signed","type":"i128"},{"name":"ratio","type":"f32"},{"name":"precise_ratio","type":"f64"},{"name":"title","type":"string"},{"name":"blob","type":"bytes"},{"name":"owner","type":"pubkey"},{"name":"scores","type":{"vec":"u64"}},{"name":"fixed_window","type":{"array":["u8",32]}},{"name":"maybe_note","type":{"option":"string"}},{"name":"nested","type":{"defined":{"name":"Inner"}}},{"name":"maybe_nested","type":{"option":{"defined":{"name":"Inner"}}}},{"name":"favorite_color","type":{"defined":{"name":"Color"}}},{"name":"color_history","type":{"vec":{"defined":{"name":"Color"}}}},{"name":"default","type":"bool"}]}}]} as const

export enum Color {
  Red = 0,
  Green = 1,
  Blue = 2,
}

export const colorCodec = getEnumCodec(Color)

export type Inner = {
  id: number
  label: string
}

export const innerCodec = getStructCodec([
  ['id', getU32Codec()],
  ['label', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
])

export type Everything = {
  flag: boolean
  tiny: number
  tinySigned: number
  small: number
  smallSigned: number
  medium: number
  mediumSigned: number
  big: bigint
  bigSigned: bigint
  huge: bigint
  hugeSigned: bigint
  ratio: number
  preciseRatio: number
  title: string
  blob: Uint8Array
  owner: Address
  scores: bigint[]
  fixedWindow: number[]
  maybeNote: string | null
  nested: Inner
  maybeNested: Inner | null
  favoriteColor: Color
  colorHistory: Color[]
  default_: boolean
}

export const everythingCodec = getStructCodec([
  ['flag', getBooleanCodec()],
  ['tiny', getU8Codec()],
  ['tinySigned', getI8Codec()],
  ['small', getU16Codec()],
  ['smallSigned', getI16Codec()],
  ['medium', getU32Codec()],
  ['mediumSigned', getI32Codec()],
  ['big', getU64Codec()],
  ['bigSigned', getI64Codec()],
  ['huge', getU128Codec()],
  ['hugeSigned', getI128Codec()],
  ['ratio', getF32Codec()],
  ['preciseRatio', getF64Codec()],
  ['title', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['blob', addCodecSizePrefix(getBytesCodec(), getU32Codec())],
  ['owner', getAddressCodec()],
  ['scores', getArrayCodec(getU64Codec(), { size: getU32Codec() })],
  ['fixedWindow', getArrayCodec(getU8Codec(), { size: 32 })],
  ['maybeNote', getNullableCodec(addCodecSizePrefix(getUtf8Codec(), getU32Codec()))],
  ['nested', innerCodec],
  ['maybeNested', getNullableCodec(innerCodec)],
  ['favoriteColor', colorCodec],
  ['colorHistory', getArrayCodec(colorCodec, { size: getU32Codec() })],
  ['default_', getBooleanCodec()],
])

export class FeatureMatrix {
  readonly programId: Uint8Array

  // The program ID is baked into the IDL, so it defaults to the generated
  // const — unlike EVM bindings where the address is a runtime value.
  constructor(
    private readonly client: SolanaClient,
    programId: string | Uint8Array = FEATURE_MATRIX_PROGRAM_ID,
  ) {
    this.programId = typeof programId === 'string' ? solanaAddressToBytes(programId) : programId
  }

  /**
   * Publishes a pre-encoded Borsh payload through the CRE signer to this
   * program's on_report entrypoint via the keystone-forwarder.
   *
   * remainingAccounts must follow the keystone-forwarder account layout:
   *   - Index 0: forwarderState – the forwarder program's state account.
   *   - Index 1: forwarderAuthority – PDA derived from seeds
   *     ["forwarder", forwarderState, receiverProgram] under the forwarder program ID.
   *   - Index 2+: receiver-specific accounts required by the target program.
   *
   * The full account list is hashed (via calculateAccountsHash) into the report.
   * The on-chain forwarder strips indices 0 and 1 before CPI-ing into the
   * receiver, so they must be present and correctly ordered.
   */
  writeReport(
    runtime: Runtime<unknown>,
    payload: Uint8Array,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    const report = runtime
      .report(
        prepareSolanaReportRequest(
          encodeForwarderReport({
            accountHash: calculateAccountsHash(remainingAccounts),
            payload,
          }),
        ),
      )
      .result()

    return this.client
      .writeReport(runtime, {
        remainingAccounts: solanaAccountMetasToJson(remainingAccounts),
        receiver: bytesToHex(this.programId),
        computeConfig,
        report,
      })
      .result()
  }

  /**
   * Publishes a Borsh Vec of pre-encoded element payloads (mirrors Go's
   * WriteReportFromBorshEncodedVec). Each element must already be fully
   * serialized for one Vec item on the wire.
   */
  writeReportFromBorshEncodedVec(
    runtime: Runtime<unknown>,
    elementPayloads: Uint8Array[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, encodeBorshVecU32(elementPayloads), remainingAccounts, computeConfig)
  }

  writeReportFromInner(
    runtime: Runtime<unknown>,
    input: Inner,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(innerCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromInners(
    runtime: Runtime<unknown>,
    inputs: Inner[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(innerCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromEverything(
    runtime: Runtime<unknown>,
    input: Everything,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(everythingCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromEverythings(
    runtime: Runtime<unknown>,
    inputs: Everything[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(everythingCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }
}
