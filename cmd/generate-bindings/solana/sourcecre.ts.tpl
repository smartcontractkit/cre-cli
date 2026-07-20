// Code generated — DO NOT EDIT.
{{- if .CodecImports}}
import {
{{- range .CodecImports}}
  {{.}},
{{- end}}
} from '@solana/codecs'
{{- end}}
{{- if .UsesAddress}}
import { getAddressCodec, type Address } from '@solana/addresses'
{{- end}}
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

export const {{.ProgramIDConst}} = '{{.ProgramID}}'

export const {{.IdlConst}} = {{.IdlJSON}} as const
{{- if or .Accounts .Events}}

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
{{- end}}
{{range .Types}}
{{- if .IsEnum}}
export enum {{.Name}} {
{{- range $i, $v := .Variants}}
  {{$v.Name}} = {{$i}},
{{- end}}
}

export const {{.CodecConst}} = getEnumCodec({{.Name}})
{{- else}}
{{- if .Fields}}
export type {{.Name}} = {
{{- range .Fields}}
  {{.Name}}: {{.TSType}}
{{- end}}
}

export const {{.CodecConst}} = getStructCodec([
{{- range .Fields}}
  ['{{.Name}}', {{.CodecExpr}}],
{{- end}}
])
{{- else}}
export type {{.Name}} = Record<string, never>

export const {{.CodecConst}} = getStructCodec([])
{{- end}}
{{- end}}
{{end}}
{{- range .Accounts}}
export const {{.ConstName}} = new Uint8Array([{{.Discriminator}}])

/**
 * Decodes raw {{.Name}} account data (with its 8-byte discriminator) into {{.TypeName}}.
 * Pure helper — there is no read capability; obtain the account bytes elsewhere.
 */
export const decode{{.Name}}Account = (data: Uint8Array): {{.TypeName}} =>
  {{.CodecConst}}.decode(expectDiscriminator('account {{.Name}}', {{.ConstName}}, data)) as {{.TypeName}}
{{end}}
{{- range .Events}}
export const {{.ConstName}} = new Uint8Array([{{.Discriminator}}])

/**
 * Decodes raw {{.Name}} event data (with its 8-byte discriminator) into {{.TypeName}}.
 */
export const decode{{.Name}}Event = (data: Uint8Array): {{.TypeName}} =>
  {{.CodecConst}}.decode(expectDiscriminator('event {{.Name}}', {{.ConstName}}, data)) as {{.TypeName}}
{{end}}
{{- if .Accounts}}
export const parseAnyAccount = (data: Uint8Array): {{range $i, $a := .Accounts}}{{if $i}} | {{end}}{{$a.TypeName}}{{end}} => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE)
  const matches = (expected: Uint8Array) => expected.every((b, i) => disc[i] === b)
{{- range .Accounts}}
  if (matches({{.ConstName}})) return decode{{.Name}}Account(data)
{{- end}}
  throw new Error(`unknown account discriminator: [${Array.from(disc).join(', ')}]`)
}
{{- end}}
{{- if .Events}}

export const parseAnyEvent = (data: Uint8Array): {{range $i, $e := .Events}}{{if $i}} | {{end}}{{$e.TypeName}}{{end}} => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE)
  const matches = (expected: Uint8Array) => expected.every((b, i) => disc[i] === b)
{{- range .Events}}
  if (matches({{.ConstName}})) return decode{{.Name}}Event(data)
{{- end}}
  throw new Error(`unknown event discriminator: [${Array.from(disc).join(', ')}]`)
}
{{- end}}
{{- if or .Accounts .Events}}
{{end}}
export class {{.ClassName}} {
  readonly programId: Uint8Array

  // The program ID is baked into the IDL, so it defaults to the generated
  // const — unlike EVM bindings where the address is a runtime value.
  constructor(
    private readonly client: SolanaClient,
    programId: string | Uint8Array = {{.ProgramIDConst}},
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
   * The full account list (indices 0..n) is hashed via calculateAccountsHash
   * into the report — this must match what the on-chain forwarder hashes, which
   * is [forwarderState, forwarderAuthority, ...receiverAccounts].
   *
   * The forwarder does NOT strip indices 0/1; it re-derives forwarderState and
   * forwarderAuthority from its own accounts and PREPENDS them to the accounts
   * it receives before CPI-ing into the receiver. So we send only indices 2+
   * (the receiver-specific accounts) — sending 0/1 too would double them and
   * break the account-hash check (InvalidAccountHash).
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
        remainingAccounts: solanaAccountMetasToJson(remainingAccounts.slice(2)),
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
{{- range .StructTypes}}

  writeReportFrom{{.Name}}(
    runtime: Runtime<unknown>,
    input: {{.Name}},
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(runtime, new Uint8Array({{.CodecConst}}.encode(input)), remainingAccounts, computeConfig)
  }

  writeReportFrom{{.Name}}s(
    runtime: Runtime<unknown>,
    inputs: {{.Name}}[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array({{.CodecConst}}.encode(input))),
      remainingAccounts,
      computeConfig,
    )
  }
{{- end}}
}
