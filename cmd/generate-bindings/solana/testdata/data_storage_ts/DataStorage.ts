// Code generated — DO NOT EDIT.
import {
  addCodecSizePrefix,
  getArrayCodec,
  getBytesCodec,
  getStructCodec,
  getU32Codec,
  getU64Codec,
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

export const DATA_STORAGE_PROGRAM_ID = 'ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL'

export const DATA_STORAGE_IDL = {"address":"ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL","metadata":{"name":"data_storage","version":"0.1.0","spec":"0.1.0","description":"Created with Anchor"},"instructions":[{"name":"get_multiple_reserves","discriminator":[104,122,140,104,175,151,70,42],"accounts":[],"args":[],"returns":{"vec":{"defined":{"name":"UpdateReserves"}}}},{"name":"get_reserves","discriminator":[121,140,237,84,218,105,48,17],"accounts":[],"args":[],"returns":{"defined":{"name":"UpdateReserves"}}},{"name":"get_tuple_reserves","discriminator":[189,83,186,20,127,80,109,49],"accounts":[],"args":[]},{"name":"initialize_data_account","discriminator":[9,64,78,49,71,193,15,250],"accounts":[{"name":"data_account","writable":true,"pda":{"seeds":[{"kind":"const","value":[100,97,116,97,95,97,99,99,111,117,110,116]},{"kind":"account","path":"user"}]}},{"name":"user","writable":true,"signer":true},{"name":"system_program","address":"11111111111111111111111111111111"}],"args":[{"name":"input","type":{"defined":{"name":"UserData"}}}]},{"name":"log_access","discriminator":[196,55,194,24,5,224,161,204],"accounts":[{"name":"user","signer":true}],"args":[{"name":"message","type":"string"}]},{"name":"on_report","discriminator":[214,173,18,221,173,148,151,208],"accounts":[{"name":"user","writable":true,"signer":true},{"name":"data_account","writable":true,"pda":{"seeds":[{"kind":"const","value":[100,97,116,97,95,97,99,99,111,117,110,116]},{"kind":"account","path":"user"}]}},{"name":"system_program","address":"11111111111111111111111111111111"}],"args":[{"name":"_metadata","type":"bytes"},{"name":"payload","type":"bytes"}]},{"name":"update_key_value_data","discriminator":[67,137,144,35,210,126,254,79],"accounts":[{"name":"user","writable":true,"signer":true},{"name":"data_account","writable":true,"pda":{"seeds":[{"kind":"const","value":[100,97,116,97,95,97,99,99,111,117,110,116]},{"kind":"account","path":"user"}]}}],"args":[{"name":"key","type":"string"},{"name":"value","type":"string"}]},{"name":"update_user_data","discriminator":[11,13,114,150,194,224,192,78],"accounts":[{"name":"user","writable":true,"signer":true},{"name":"data_account","writable":true,"pda":{"seeds":[{"kind":"const","value":[100,97,116,97,95,97,99,99,111,117,110,116]},{"kind":"account","path":"user"}]}}],"args":[{"name":"input","type":{"defined":{"name":"UserData"}}}]}],"accounts":[{"name":"DataAccount","discriminator":[85,240,182,158,76,7,18,233]}],"events":[{"name":"AccessLogged","discriminator":[243,53,225,71,64,120,109,25]},{"name":"DynamicEvent","discriminator":[236,145,224,161,9,222,218,237]},{"name":"NoFields","discriminator":[160,156,94,85,77,122,98,240]}],"errors":[{"code":6000,"name":"DataNotFound","msg":"data not found"}],"types":[{"name":"AccessLogged","type":{"kind":"struct","fields":[{"name":"caller","type":"pubkey"},{"name":"message","type":"string"}]}},{"name":"DataAccount","type":{"kind":"struct","fields":[{"name":"sender","type":"string"},{"name":"key","type":"string"},{"name":"value","type":"string"}]}},{"name":"DynamicEvent","type":{"kind":"struct","fields":[{"name":"key","type":"string"},{"name":"user_data","type":{"defined":{"name":"UserData"}}},{"name":"sender","type":"string"},{"name":"metadata","type":"bytes"},{"name":"metadata_array","type":{"vec":"bytes"}}]}},{"name":"NoFields","type":{"kind":"struct","fields":[]}},{"name":"UpdateReserves","type":{"kind":"struct","fields":[{"name":"total_minted","type":"u64"},{"name":"total_reserve","type":"u64"}]}},{"name":"UserData","type":{"kind":"struct","fields":[{"name":"key","type":"string"},{"name":"value","type":"string"}]}}]} as const

const DISCRIMINATOR_SIZE = 8

const expectDiscriminator = (label: string, expected: Uint8Array, data: Uint8Array): Uint8Array => {
  if (data.length < DISCRIMINATOR_SIZE) {
    throw new Error(`${label}: data too short for discriminator (${data.length} bytes)`)
  }
  for (let i = 0; i < DISCRIMINATOR_SIZE; i++) {
    if (data[i] !== expected[i]) {
      throw new Error(`${label}: discriminator mismatch`)
    }
  }
  return data.subarray(DISCRIMINATOR_SIZE)
}

export type AccessLogged = {
  caller: Address
  message: string
}

export const accessLoggedCodec = getStructCodec([
  ['caller', getAddressCodec()],
  ['message', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
])

export type DataAccount = {
  sender: string
  key: string
  value: string
}

export const dataAccountCodec = getStructCodec([
  ['sender', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['key', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['value', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
])

export type UserData = {
  key: string
  value: string
}

export const userDataCodec = getStructCodec([
  ['key', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['value', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
])

export type DynamicEvent = {
  key: string
  userData: UserData
  sender: string
  metadata: Uint8Array
  metadataArray: Uint8Array[]
}

export const dynamicEventCodec = getStructCodec([
  ['key', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['userData', userDataCodec],
  ['sender', addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ['metadata', addCodecSizePrefix(getBytesCodec(), getU32Codec())],
  ['metadataArray', getArrayCodec(addCodecSizePrefix(getBytesCodec(), getU32Codec()), { size: getU32Codec() })],
])

export type NoFields = Record<string, never>

export const noFieldsCodec = getStructCodec([])

export type UpdateReserves = {
  totalMinted: bigint
  totalReserve: bigint
}

export const updateReservesCodec = getStructCodec([
  ['totalMinted', getU64Codec()],
  ['totalReserve', getU64Codec()],
])

export const ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR = new Uint8Array([85, 240, 182, 158, 76, 7, 18, 233])

/**
 * Decodes raw DataAccount account data (with its 8-byte discriminator) into DataAccount.
 * Pure helper — there is no read capability; obtain the account bytes elsewhere.
 */
export const decodeDataAccountAccount = (data: Uint8Array): DataAccount =>
  dataAccountCodec.decode(expectDiscriminator('account DataAccount', ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR, data)) as DataAccount

export const EVENT_ACCESS_LOGGED_DISCRIMINATOR = new Uint8Array([243, 53, 225, 71, 64, 120, 109, 25])

/**
 * Decodes raw AccessLogged event data (with its 8-byte discriminator) into AccessLogged.
 */
export const decodeAccessLoggedEvent = (data: Uint8Array): AccessLogged =>
  accessLoggedCodec.decode(expectDiscriminator('event AccessLogged', EVENT_ACCESS_LOGGED_DISCRIMINATOR, data)) as AccessLogged

export const EVENT_DYNAMIC_EVENT_DISCRIMINATOR = new Uint8Array([236, 145, 224, 161, 9, 222, 218, 237])

/**
 * Decodes raw DynamicEvent event data (with its 8-byte discriminator) into DynamicEvent.
 */
export const decodeDynamicEventEvent = (data: Uint8Array): DynamicEvent =>
  dynamicEventCodec.decode(expectDiscriminator('event DynamicEvent', EVENT_DYNAMIC_EVENT_DISCRIMINATOR, data)) as DynamicEvent

export const EVENT_NO_FIELDS_DISCRIMINATOR = new Uint8Array([160, 156, 94, 85, 77, 122, 98, 240])

/**
 * Decodes raw NoFields event data (with its 8-byte discriminator) into NoFields.
 */
export const decodeNoFieldsEvent = (data: Uint8Array): NoFields =>
  noFieldsCodec.decode(expectDiscriminator('event NoFields', EVENT_NO_FIELDS_DISCRIMINATOR, data)) as NoFields

export const parseAnyAccount = (data: Uint8Array): DataAccount => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE)
  const matches = (expected: Uint8Array) => expected.every((b, i) => disc[i] === b)
  if (matches(ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR)) return decodeDataAccountAccount(data)
  throw new Error(`unknown account discriminator: [${Array.from(disc).join(', ')}]`)
}

export const parseAnyEvent = (data: Uint8Array): AccessLogged | DynamicEvent | NoFields => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE)
  const matches = (expected: Uint8Array) => expected.every((b, i) => disc[i] === b)
  if (matches(EVENT_ACCESS_LOGGED_DISCRIMINATOR)) return decodeAccessLoggedEvent(data)
  if (matches(EVENT_DYNAMIC_EVENT_DISCRIMINATOR)) return decodeDynamicEventEvent(data)
  if (matches(EVENT_NO_FIELDS_DISCRIMINATOR)) return decodeNoFieldsEvent(data)
  throw new Error(`unknown event discriminator: [${Array.from(disc).join(', ')}]`)
}

export class DataStorage {
  readonly programId: Uint8Array

  // The program ID is baked into the IDL, so it defaults to the generated
  // const — unlike EVM bindings where the address is a runtime value.
  constructor(
    private readonly client: SolanaClient,
    programId: string | Uint8Array = DATA_STORAGE_PROGRAM_ID,
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

  writeReportFromAccessLogged(
    runtime: Runtime<unknown>,
    input: AccessLogged,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(accessLoggedCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromAccessLoggeds(
    runtime: Runtime<unknown>,
    inputs: AccessLogged[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(accessLoggedCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromDataAccount(
    runtime: Runtime<unknown>,
    input: DataAccount,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(dataAccountCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromDataAccounts(
    runtime: Runtime<unknown>,
    inputs: DataAccount[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(dataAccountCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromUserData(
    runtime: Runtime<unknown>,
    input: UserData,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(userDataCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromUserDatas(
    runtime: Runtime<unknown>,
    inputs: UserData[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(userDataCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromDynamicEvent(
    runtime: Runtime<unknown>,
    input: DynamicEvent,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(dynamicEventCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromDynamicEvents(
    runtime: Runtime<unknown>,
    inputs: DynamicEvent[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(dynamicEventCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromNoFields(
    runtime: Runtime<unknown>,
    input: NoFields,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(noFieldsCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromNoFieldss(
    runtime: Runtime<unknown>,
    inputs: NoFields[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(noFieldsCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }

  writeReportFromUpdateReserves(
    runtime: Runtime<unknown>,
    input: UpdateReserves,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array(updateReservesCodec.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFromUpdateReservess(
    runtime: Runtime<unknown>,
    inputs: UpdateReserves[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(updateReservesCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }
}
