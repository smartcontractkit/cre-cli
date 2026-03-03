// Code generated — DO NOT EDIT.
import type { Address } from 'viem'
import { addContractMock, type ContractMock, type EvmMock } from '@chainlink/cre-sdk/test'
{{range $contract := .Contracts}}
import { {{$contract.Type}}ABI } from './{{$contract.Type}}'

export type {{$contract.Type}}Mock = {
  {{- range $call := $contract.Calls}}
  {{- if or $call.Original.Constant (eq $call.Original.StateMutability "view") (eq $call.Original.StateMutability "pure")}}
  {{decapitalise $call.Normalized.Name}}?: ({{range $idx, $param := $call.Normalized.Inputs}}{{if $idx}}, {{end}}{{$param.Name}}: {{bindtype $param.Type $.Structs}}{{end}}) => {{returntype $call.Normalized.Outputs $.Structs}}
  {{- end}}
  {{- end}}
} & Pick<ContractMock<typeof {{$contract.Type}}ABI>, 'writeReport'>

export function new{{$contract.Type}}Mock(address: Address, evmMock: EvmMock): {{$contract.Type}}Mock {
  return addContractMock(evmMock, { address, abi: {{$contract.Type}}ABI }) as {{$contract.Type}}Mock
}
{{end}}
