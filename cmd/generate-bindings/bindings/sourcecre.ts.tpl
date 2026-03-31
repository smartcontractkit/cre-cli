// Code generated — DO NOT EDIT.
import {
  decodeEventLog,
  decodeFunctionResult,
  encodeEventTopics,
  encodeFunctionData,
  zeroAddress,
} from 'viem'
import type { Address, Hex } from 'viem'
import {
  bytesToHex,
  encodeCallMsg,
  EVMClient,
  hexToBase64,
  LAST_FINALIZED_BLOCK_NUMBER,
  prepareReportRequest,
  type EVMLog,
  type Runtime,
} from '@chainlink/cre-sdk'

export interface DecodedLog<T> extends Omit<EVMLog, 'data'> { data: T }

const encodeTopicValue = (t: Hex | Hex[] | null): string[] => {
  if (t == null) return []
  if (Array.isArray(t)) return t.map(hexToBase64)
  return [hexToBase64(t)]
}

{{range $contract := .Contracts}}
{{/* Event types: Topics (indexed only) and Decoded (all fields) */}}
{{range $event := $contract.Events}}

/**
 * Filter params for {{.Original.Name}}. Only indexed fields can be used for filtering.
 * Indexed string/bytes must be passed as keccak256 hash (Hex).
 */
export type {{.Normalized.Name}}Topics = {
  {{- range .Normalized.Inputs}}
  {{- if .Indexed}}
  {{.Name}}?: {{bindtopictype .Type $.Structs}}
  {{- end}}
  {{- end}}
}

/**
 * Decoded {{.Original.Name}} event data.
 */
export type {{.Normalized.Name}}Decoded = {
  {{- range .Normalized.Inputs}}
  {{.Name}}: {{bindtype .Type $.Structs}}
  {{- end}}
}
{{end}}

export const {{$contract.Type}}ABI = {{unescapeabi .InputABI}} as const

export class {{$contract.Type}} {
  constructor(
    private readonly client: EVMClient,
    public readonly address: Address,
  ) {}

  {{- range $call := $contract.Calls}}
  {{- if or $call.Original.Constant (eq $call.Original.StateMutability "view") (eq $call.Original.StateMutability "pure")}}

  {{decapitalise $call.Normalized.Name}}(
    runtime: Runtime<unknown>,
    {{- range $param := $call.Normalized.Inputs}}
    {{$param.Name}}: {{bindtype $param.Type $.Structs}},
    {{- end}}
  ): {{returntype $call.Normalized.Outputs $.Structs}} {
    const callData = encodeFunctionData({
      abi: {{$contract.Type}}ABI,
      functionName: '{{$call.Original.Name}}' as const,
      {{- if gt (len $call.Normalized.Inputs) 0}}
      args: [{{range $idx, $param := $call.Normalized.Inputs}}{{if $idx}}, {{end}}{{$param.Name}}{{end}}],
      {{- end}}
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: {{$contract.Type}}ABI,
      functionName: '{{$call.Original.Name}}' as const,
      data: bytesToHex(result.data),
    }) as {{returntype $call.Normalized.Outputs $.Structs}}
  }
  {{- end}}
  {{- end}}

  {{- range $call := $contract.Calls}}
  {{- if not (or $call.Original.Constant (eq $call.Original.StateMutability "view") (eq $call.Original.StateMutability "pure"))}}
  {{- if gt (len $call.Normalized.Inputs) 0}}

  writeReportFrom{{capitalise $call.Normalized.Name}}(
    runtime: Runtime<unknown>,
    {{- range $param := $call.Normalized.Inputs}}
    {{$param.Name}}: {{bindtype $param.Type $.Structs}},
    {{- end}}
    gasConfig?: { gasLimit?: string },
  ) {
    const callData = encodeFunctionData({
      abi: {{$contract.Type}}ABI,
      functionName: '{{$call.Original.Name}}' as const,
      args: [{{range $idx, $param := $call.Normalized.Inputs}}{{if $idx}}, {{end}}{{$param.Name}}{{end}}],
    })

    const reportResponse = runtime
      .report(prepareReportRequest(callData))
      .result()

    return this.client
      .writeReport(runtime, {
        receiver: this.address,
        report: reportResponse,
        gasConfig,
      })
      .result()
  }
  {{- end}}
  {{- end}}
  {{- end}}

  writeReport(
    runtime: Runtime<unknown>,
    callData: Hex,
    gasConfig?: { gasLimit?: string },
  ) {
    const reportResponse = runtime
      .report(prepareReportRequest(callData))
      .result()

    return this.client
      .writeReport(runtime, {
        receiver: this.address,
        report: reportResponse,
        gasConfig,
      })
      .result()
  }
{{- range $event := $contract.Events}}

  /**
   * Creates a log trigger for {{.Original.Name}} events.
   * The returned trigger's adapt method decodes the raw log into {{.Normalized.Name}}Decoded,
   * so the handler receives typed event data directly.
   * When multiple filters are provided, topic values are merged with OR semantics (match any).
   */
  logTrigger{{.Normalized.Name}}(
    filters?: {{.Normalized.Name}}Topics[],
  ) {
    let topics: { values: string[] }[]
    if (!filters || filters.length === 0) {
      const encoded = encodeEventTopics({
        abi: {{$contract.Type}}ABI,
        eventName: '{{.Original.Name}}' as const,
      })
      topics = encoded.map((t) => ({ values: encodeTopicValue(t) }))
    } else if (filters.length === 1) {
      const f = filters[0]
      const args = {
        {{- range $param := .Normalized.Inputs}}
        {{- if $param.Indexed}}
        {{$param.Name}}: f.{{$param.Name}},
        {{- end}}
        {{- end}}
      }
      const encoded = encodeEventTopics({
        abi: {{$contract.Type}}ABI,
        eventName: '{{.Original.Name}}' as const,
        args,
      })
      topics = encoded.map((t) => ({ values: encodeTopicValue(t) }))
    } else {
      const allEncoded = filters.map((f) => {
        const args = {
          {{- range $param := .Normalized.Inputs}}
          {{- if $param.Indexed}}
          {{$param.Name}}: f.{{$param.Name}},
          {{- end}}
          {{- end}}
        }
        return encodeEventTopics({
          abi: {{$contract.Type}}ABI,
          eventName: '{{.Original.Name}}' as const,
          args,
        })
      })
      topics = allEncoded[0].map((_, i) => ({
        values: [...new Set(allEncoded.flatMap((row) => encodeTopicValue(row[i])))],
      }))
    }
    const baseTrigger = this.client.logTrigger({
      addresses: [hexToBase64(this.address)],
      topics,
    })
    const contract = this
    return {
      capabilityId: () => baseTrigger.capabilityId(),
      method: () => baseTrigger.method(),
      outputSchema: () => baseTrigger.outputSchema(),
      configAsAny: () => baseTrigger.configAsAny(),
      adapt: (rawOutput: EVMLog): DecodedLog<{{.Normalized.Name}}Decoded> => contract.decode{{.Normalized.Name}}(rawOutput),
    }
  }

  /**
   * Decodes a log into {{.Normalized.Name}} data, preserving all log metadata.
   */
  decode{{.Normalized.Name}}(log: EVMLog): DecodedLog<{{.Normalized.Name}}Decoded> {
    const decoded = decodeEventLog({
      abi: {{$contract.Type}}ABI,
      data: bytesToHex(log.data),
      topics: log.topics.map((t) => bytesToHex(t)) as [Hex, ...Hex[]],
    })
    const { data: _, ...rest } = log
    return { ...rest, data: decoded.args as unknown as {{.Normalized.Name}}Decoded }
  }
{{- end}}
}
{{end}}
