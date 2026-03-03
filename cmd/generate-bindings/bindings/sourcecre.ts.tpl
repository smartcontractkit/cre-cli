// Code generated — DO NOT EDIT.
import { decodeFunctionResult, encodeFunctionData, zeroAddress } from 'viem'
import type { Address, Hex } from 'viem'
import {
  bytesToHex,
  encodeCallMsg,
  EVMClient,
  hexToBase64,
  LAST_FINALIZED_BLOCK_NUMBER,
  prepareReportRequest,
  type Runtime,
} from '@chainlink/cre-sdk'

{{range $contract := .Contracts}}
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
}
{{end}}
