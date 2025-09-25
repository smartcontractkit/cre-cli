// Code generated — DO NOT EDIT.

package {{.Package}}

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	evmmock "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm/mock"
)

var (
	_ = errors.New
	_ = fmt.Errorf
	_ = big.NewInt
	_ = common.Big1
)

{{range $contract := .Contracts}}
// {{$contract.Type}}Mock is a mock implementation of {{$contract.Type}} for testing.
type {{$contract.Type}}Mock struct {
	{{- range $call := $contract.Calls}}
	{{- if or $call.Original.Constant (eq $call.Original.StateMutability "view")}}
	{{$call.Normalized.Name}} func({{- if gt (len $call.Normalized.Inputs) 0}}{{- if gt (len $call.Normalized.Inputs) 0}}{{$call.Normalized.Name}}Input{{- end}}{{- end}}) ({{- if gt (len $call.Normalized.Outputs) 1 -}}{{$call.Normalized.Name}}Output{{- else if eq (len $call.Normalized.Outputs) 1 -}}{{with index $call.Normalized.Outputs 0}}{{bindtype .Type $.Structs}}{{end}}{{- else -}}error{{- end}}, error)
	{{- end}}
	{{- end}}
}

// New{{$contract.Type}}Mock creates a new {{$contract.Type}}Mock for testing.
func New{{$contract.Type}}Mock(address common.Address, clientMock *evmmock.ClientCapability) *{{$contract.Type}}Mock {
	mock := &{{$contract.Type}}Mock{}
	
	codec, err := NewCodec()
	if err != nil {
		panic("failed to create codec for mock: " + err.Error())
	}
	
	abi := codec.(*Codec).abi
	_ = abi
	
	funcMap := map[string]func([]byte) ([]byte, error){
		{{- range $call := $contract.Calls}}
		{{- if or $call.Original.Constant (eq $call.Original.StateMutability "view")}}
		string(abi.Methods["{{$call.Original.Name}}"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.{{$call.Normalized.Name}} == nil {
				return nil, errors.New("{{$call.Original.Name}} method not mocked")
			}
			
			{{- if gt (len $call.Normalized.Inputs) 0}}
			inputs := abi.Methods["{{$call.Original.Name}}"].Inputs
			
			values, err := inputs.Unpack(payload)
			if err != nil {
				return nil, errors.New("Failed to unpack payload")
			}
			
			{{- if gt (len $call.Normalized.Inputs) 1}}
			if len(values) != {{len $call.Normalized.Inputs}} {
				return nil, errors.New("expected {{len $call.Normalized.Inputs}} input values")
			}
			
			args := {{$call.Normalized.Name}}Input{
				{{- range $idx, $param := $call.Normalized.Inputs}}
				{{capitalise $param.Name}}: values[{{$idx}}].({{bindtype $param.Type $.Structs}}),
				{{- end}}
			}
			
			result, err := mock.{{$call.Normalized.Name}}(args)
			{{- else}}
			if len(values) != 1 {
				return nil, errors.New("expected 1 input value")
			}
			
			args := {{$call.Normalized.Name}}Input{
				{{- with index $call.Normalized.Inputs 0}}
				{{capitalise .Name}}: values[0].({{bindtype .Type $.Structs}}),
				{{- end}}
			}
			
			result, err := mock.{{$call.Normalized.Name}}(args)
			{{- end}}
			{{- else}}
			result, err := mock.{{$call.Normalized.Name}}()
			{{- end}}
			if err != nil {
				return nil, err
			}
			
			{{- if gt (len $call.Normalized.Outputs) 1}}
			return abi.Methods["{{$call.Original.Name}}"].Outputs.Pack(
				{{- range $idx, $param := $call.Normalized.Outputs}}
				{{- if $param.Name}}
				result.{{capitalise $param.Name}},
				{{- else}}
				result.Output{{$idx}},
				{{- end}}
				{{- end}}
			)
			{{- else if eq (len $call.Normalized.Outputs) 1}}
			return abi.Methods["{{$call.Original.Name}}"].Outputs.Pack(result)
			{{- else}}
			return nil, nil
			{{- end}}
		},
		{{- end}}
		{{- end}}
	}
	
	evmmock.AddContractMock(address, clientMock, funcMap, nil)
	return mock
}
{{end}}