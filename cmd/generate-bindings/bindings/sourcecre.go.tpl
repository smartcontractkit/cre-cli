// Code generated — DO NOT EDIT.

package {{.Package}}

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm/bindings"
    "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
    pb2 "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
)

var (
	_ = bytes.Equal
	_ = errors.New
	_ = fmt.Sprintf
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
	_ = emptypb.Empty{}
	_ = pb.NewBigIntFromInt
	_ = pb2.AggregationType_AGGREGATION_TYPE_COMMON_PREFIX
	_ = bindings.FilterOptions{}
	_ = evm.FilterLogTriggerRequest{}
	_ = cre.ResponseBufferTooSmall
	_ = rpc.API{}
	_ = json.Unmarshal
)

{{range $contract := .Contracts}}
var {{$contract.Type}}MetaData = &bind.MetaData{
	ABI: "{{.InputABI}}",
	{{- if .InputBin}}
	Bin: "0x{{.InputBin}}",
	{{- end}}
}

// Structs 
{{range $.Structs}}type {{.Name}} struct {
	{{- range .Fields}}
	{{capitalise .Name}} {{.Type}}
	{{- end}}
}

{{end}}

// Contract Method Inputs{{- range $call := $contract.Calls}}
{{- if gt (len $call.Normalized.Inputs) 0 }}
type {{$call.Normalized.Name}}Input struct {
	{{- range $param := $call.Normalized.Inputs}}
	{{capitalise $param.Name}} {{bindtype .Type $.Structs}}
	{{- end}}
}
{{end}}

{{- end}}

// Contract Method Outputs{{- range $call := $contract.Calls}}
{{- if gt (len $call.Normalized.Outputs) 1 }}
type {{$call.Normalized.Name}}Output struct {
	{{- range $idx, $param := $call.Normalized.Outputs}}
	{{- if $param.Name}}
	{{capitalise $param.Name}} {{bindtype .Type $.Structs}}
	{{- else}}
	Output{{$idx}} {{bindtype .Type $.Structs}}
	{{- end}}
	{{- end}}
}
{{end}}

{{- end}}

// Errors
{{range $error := $contract.Errors}}type {{.Normalized.Name}} struct {
	{{- range .Normalized.Inputs}}
	{{capitalise .Name}} {{bindtype .Type $.Structs}}
	{{- end}}
}

{{end}}

// Events
// The <Event>Topics struct should be used as a filter (for log triggers).
// Note: It is only possible to filter on indexed fields.
// Indexed (string and bytes) fields will be of type common.Hash.
// They need to he (crypto.Keccak256) hashed and passed in.
// Indexed (tuple/slice/array) fields can be passed in as is, the Encode<Event>Topics function will handle the hashing.
//
// The <Event>Decoded struct will be the result of calling decode (Adapt) on the log trigger result.
// Indexed dynamic type fields will be of type common.Hash.
{{range $event := $contract.Events}}

type {{.Normalized.Name}}Topics struct {
	{{- range .Normalized.Inputs}}
	{{- if .Indexed}}
	{{capitalise .Name}} {{bindtopictype .Type $.Structs}}
	{{- end}}
	{{- end}}
}

type {{.Normalized.Name}}Decoded struct {
	{{- range .Normalized.Inputs}}
	{{capitalise .Name}} {{if and .Indexed (isDynTopicType .Type)}}common.Hash{{else}}{{bindtype .Type $.Structs}}{{end}}
	{{- end}}
}

{{end}}

// Main Binding Type for {{$contract.Type}}
type {{$contract.Type}} struct {
	Address   common.Address
	Options   *bindings.ContractInitOptions
	ABI       *abi.ABI
	client *evm.Client
	Codec     {{$contract.Type}}Codec
}

type {{$contract.Type}}Codec interface {
	{{- range $call := $contract.Calls}}
	
	{{- if gt (len $call.Normalized.Inputs) 0 }}
	Encode{{$call.Normalized.Name}}MethodCall(in {{$call.Normalized.Name}}Input) ([]byte, error)
	{{- else }}
	Encode{{$call.Normalized.Name}}MethodCall() ([]byte, error)
	{{- end }}
	{{- if gt (len $call.Normalized.Outputs) 1 }}
	Decode{{$call.Normalized.Name}}MethodOutput(data []byte) ({{$call.Normalized.Name}}Output, error)
	{{- else if eq (len $call.Normalized.Outputs) 1 }}
	Decode{{$call.Normalized.Name}}MethodOutput(data []byte) ({{with index $call.Normalized.Outputs 0}}{{bindtype .Type $.Structs}}{{end}}, error)
	{{- end }}
	
	{{- end}}

	{{- range $.Structs}}
	Encode{{.Name}}Struct(in {{.Name}}) ([]byte, error)
	{{- end}}

	{{- range $event := .Events}}
	{{.Normalized.Name}}LogHash() []byte
	Encode{{.Normalized.Name}}Topics(evt abi.Event, values []{{.Normalized.Name}}Topics) ([]*evm.TopicValues, error)
	Decode{{.Normalized.Name}}(log *evm.Log) (*{{.Normalized.Name}}Decoded, error)
	{{- end}}
}

func New{{$contract.Type}}(
	client *evm.Client,
	address common.Address,
	options *bindings.ContractInitOptions,
) (*{{$contract.Type}}, error) {
	parsed, err := abi.JSON(strings.NewReader({{$contract.Type}}MetaData.ABI))
	if err != nil {
		return nil, err
	}
	codec, err := NewCodec()
	if err != nil {
		return nil, err
	}
	return &{{$contract.Type}}{
		Address:   address,
		Options:   options,
		ABI:       &parsed,
		client: client,
		Codec:     codec,
	}, nil
}

type Codec struct {
	abi *abi.ABI
}

func NewCodec() ({{$contract.Type}}Codec, error) {
	parsed, err := abi.JSON(strings.NewReader({{$contract.Type}}MetaData.ABI))
	if err != nil {
		return nil, err
	}
	return &Codec{abi: &parsed}, nil
}

{{range $call := $contract.Calls}}

{{- if gt (len $call.Normalized.Inputs) 0 }}

func (c *Codec) Encode{{ $call.Normalized.Name }}MethodCall(in {{ $call.Normalized.Name }}Input) ([]byte, error) {
	return c.abi.Pack("{{ $call.Original.Name }}"{{- range .Normalized.Inputs }}, in.{{ capitalise .Name }}{{- end }})
}
{{- else }}
func (c *Codec) Encode{{ $call.Normalized.Name }}MethodCall() ([]byte, error) {
	return c.abi.Pack("{{ $call.Original.Name }}")
}

{{- end }}

{{- if gt (len $call.Normalized.Outputs) 1 }}

func (c *Codec) Decode{{ $call.Normalized.Name }}MethodOutput(data []byte) ({{ $call.Normalized.Name }}Output, error) {
	vals, err := c.abi.Methods["{{ $call.Original.Name }}"].Outputs.Unpack(data)
	if err != nil {
		return {{ $call.Normalized.Name }}Output{}, err
	}
	if len(vals) != {{ len $call.Normalized.Outputs }} {
		return {{ $call.Normalized.Name }}Output{}, fmt.Errorf("expected {{ len $call.Normalized.Outputs }} values, got %d", len(vals))
	}

	{{- range $idx, $param := $call.Normalized.Outputs}}
	jsonData{{ $idx }}, err := json.Marshal(vals[{{ $idx }}])
	if err != nil {
		return {{ $call.Normalized.Name }}Output{}, fmt.Errorf("failed to marshal ABI result {{ $idx }}: %w", err)
	}

	var result{{ $idx }} {{ bindtype $param.Type $.Structs }}
	if err := json.Unmarshal(jsonData{{ $idx }}, &result{{ $idx }}); err != nil {
		return {{ $call.Normalized.Name }}Output{}, fmt.Errorf("failed to unmarshal to {{ bindtype $param.Type $.Structs }}: %w", err)
	}
	{{- end}}

	return {{ $call.Normalized.Name }}Output{
		{{- range $idx, $param := $call.Normalized.Outputs}}
		{{- if $param.Name}}
		{{capitalise $param.Name}}: result{{ $idx }},
		{{- else}}
		Output{{ $idx }}: result{{ $idx }},
		{{- end}}
		{{- end}}
	}, nil
}
{{- else if eq (len $call.Normalized.Outputs) 1 }}

func (c *Codec) Decode{{ $call.Normalized.Name }}MethodOutput(data []byte) ({{ with index $call.Normalized.Outputs 0 }}{{ bindtype .Type $.Structs }}{{ end }}, error) {
	vals, err := c.abi.Methods["{{ $call.Original.Name }}"].Outputs.Unpack(data)
	if err != nil {
		return {{ with index $call.Normalized.Outputs 0 }}*new({{ bindtype .Type $.Structs }}){{ end }}, err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return {{ with index $call.Normalized.Outputs 0 }}*new({{ bindtype .Type $.Structs }}){{ end }}, fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result {{ bindtype (index $call.Normalized.Outputs 0).Type $.Structs }}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return {{ with index $call.Normalized.Outputs 0 }}*new({{ bindtype .Type $.Structs }}){{ end }}, fmt.Errorf("failed to unmarshal to {{ bindtype (index $call.Normalized.Outputs 0).Type $.Structs }}: %w", err)
	}

	return result, nil
}
{{- end }}

{{end}}

{{range $.Structs}}
func (c *Codec) Encode{{.Name}}Struct(in {{.Name}}) ([]byte, error) {
	tupleType, err := abi.NewType(
        "tuple", "",
        []abi.ArgumentMarshaling{
			{{range $f := .Fields}}{Name: "{{ decapitalise $f.Name }}", Type: "{{ $f.SolKind }}"},
			{{end}}
        },
    )
	if err != nil {
		return nil, fmt.Errorf("failed to create tuple type for {{.Name}}: %w", err)
	}
	args := abi.Arguments{
        {Name: "{{ decapitalise .Name }}", Type: tupleType},
    }

	return args.Pack(in)
}
{{- end }}

{{range $event := $contract.Events}}
func (c *Codec) {{.Normalized.Name}}LogHash() []byte {
	return c.abi.Events["{{.Original.Name}}"].ID.Bytes()
}

func (c *Codec) Encode{{.Normalized.Name}}Topics(
    evt abi.Event,
    values []{{.Normalized.Name}}Topics,
) ([]*evm.TopicValues, error) {
    {{- range $idx, $inp := .Normalized.Inputs }}
    {{- if $inp.Indexed }}
    var {{ decapitalise $inp.Name }}Rule []interface{}
    for _, v := range values {
		if reflect.ValueOf(v.{{capitalise $inp.Name}}).IsZero() {
			{{ decapitalise $inp.Name }}Rule = append({{ decapitalise $inp.Name }}Rule, common.Hash{})
			continue
		}
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[{{$idx}}], v.{{capitalise $inp.Name}})
		if err != nil {
			return nil, err
		}
		{{ decapitalise $inp.Name }}Rule = append({{ decapitalise $inp.Name }}Rule, fieldVal)
	}
    {{- end }}
    {{- end }}

    rawTopics, err := abi.MakeTopics(
        {{- range $inp := .Normalized.Inputs }}
        {{- if $inp.Indexed }}
        {{ decapitalise $inp.Name }}Rule,
        {{- end }}
        {{- end }}
    )
    if err != nil {
        return nil, err
    }

	topics := make([]*evm.TopicValues, len(rawTopics)+1)
	topics[0] = &evm.TopicValues{
		Values: [][]byte{evt.ID.Bytes()},
	}
    for i, hashList := range rawTopics {
        bs := make([][]byte, len(hashList))
        for j, h := range hashList {
			// don't include empty bytes if hashed value is 0x0
            if reflect.ValueOf(h).IsZero() {
				bs[j] = []byte{}
			} else {
				bs[j] = h.Bytes()
			}
        }
        topics[i+1] = &evm.TopicValues{Values: bs}
    }
    return topics, nil
}


// Decode{{.Normalized.Name}} decodes a log into a {{.Normalized.Name}} struct.
func (c *Codec) Decode{{.Normalized.Name}}(log *evm.Log) (*{{.Normalized.Name}}Decoded, error) {
	event := new({{.Normalized.Name}}Decoded)
	if err := c.abi.UnpackIntoInterface(event, "{{.Original.Name}}", log.Data); err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events["{{.Original.Name}}"].Inputs {
		if arg.Indexed {
			if arg.Type.T == abi.TupleTy {
				// abigen throws on tuple, so converting to bytes to
				// receive back the common.Hash as is instead of error
				arg.Type.T = abi.BytesTy
			}
			indexed = append(indexed, arg)
		}
	}
	// Convert [][]byte → []common.Hash
	topics := make([]common.Hash, len(log.Topics))
	for i, t := range log.Topics {
		topics[i] = common.BytesToHash(t)
	}

	if err := abi.ParseTopics(event, indexed, topics[1:]); err != nil {
		return nil, err
	}
	return event, nil
}
{{end}}

{{range $call := $contract.Calls}}
  {{- if or $call.Original.Constant (eq $call.Original.StateMutability "view")}}

func (c {{$contract.Type}}) {{$call.Normalized.Name}}(
    runtime cre.Runtime,
    {{- if gt (len $call.Normalized.Inputs) 0}}
    args {{$call.Normalized.Name}}Input,
    {{- end}}
    blockNumber *big.Int,
) {{- if gt (len $call.Normalized.Outputs) 1 -}}
cre.Promise[{{$call.Normalized.Name}}Output]
{{- else if eq (len $call.Normalized.Outputs) 1 -}}
cre.Promise[{{with index $call.Normalized.Outputs 0}}{{bindtype .Type $.Structs}}{{end}}]
{{- else -}}
cre.Promise[*evm.CallContractReply]
{{- end}} {
    {{- if gt (len $call.Normalized.Inputs) 0}}
    calldata, err := c.Codec.Encode{{$call.Normalized.Name}}MethodCall(args)
	{{- else }}
	calldata, err := c.Codec.Encode{{$call.Normalized.Name}}MethodCall()
	{{- end}}
    if err != nil {
        {{- if gt (len $call.Normalized.Outputs) 1}}
        return cre.PromiseFromResult[{{$call.Normalized.Name}}Output]({{$call.Normalized.Name}}Output{}, err)
        {{- else if eq (len $call.Normalized.Outputs) 1}}
        return cre.PromiseFromResult[{{with index $call.Normalized.Outputs 0}}{{bindtype .Type $.Structs}}{{end}}]({{with index $call.Normalized.Outputs 0}}*new({{bindtype .Type $.Structs}}){{end}}, err)
        {{- else}}
        return cre.PromiseFromResult[*evm.CallContractReply](nil, err)
        {{- end}}
    }

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: bindings.FinalizedBlockNumber,
		})

		bn = cre.Then(promise, func(finalizedBlock *evm.HeaderByNumberReply) (*pb.BigInt, error) {
            if finalizedBlock == nil || finalizedBlock.Header == nil {
                return nil, errors.New("failed to get finalized block header")
            }
            return finalizedBlock.Header.BlockNumber, nil
        })
	} else {
		bn = cre.PromiseFromResult(pb.NewBigIntFromInt(blockNumber), nil)
	}

    promise := cre.ThenPromise(bn, func(bn *pb.BigInt) cre.Promise[*evm.CallContractReply] {
        return c.client.CallContract(runtime, &evm.CallContractRequest{
            Call:        &evm.CallMsg{To: c.Address.Bytes(), Data: calldata},
            BlockNumber: bn,
        })
    })

    {{- if gt (len $call.Normalized.Outputs) 0}}
    return cre.Then(promise, func(response *evm.CallContractReply) ({{- if gt (len $call.Normalized.Outputs) 1 -}}{{$call.Normalized.Name}}Output{{- else -}}{{with index $call.Normalized.Outputs 0}}{{bindtype .Type $.Structs}}{{end}}{{- end}}, error) {
        return c.Codec.Decode{{$call.Normalized.Name}}MethodOutput(response.Data)
    })
    {{- else}}
    // No outputs to decode, return raw response
    return promise
    {{- end}}

}
  {{- end}}
{{end}}

{{range $.Structs}}

func (c {{$contract.Type}}) WriteReportFrom{{.Name}}(
	runtime cre.Runtime,
	input {{.Name}},
	gasConfig *evm.GasConfig,
) cre.Promise[*evm.WriteReportReply] {
	encoded, err := c.Codec.Encode{{.Name}}Struct(input)
	if err != nil {
		return cre.PromiseFromResult[*evm.WriteReportReply](nil, err)
	}
	promise := runtime.GenerateReport(&pb2.ReportRequest{
		EncodedPayload: encoded,
		EncoderName:    "evm",
		SigningAlgo:    "ecdsa",
		HashingAlgo:    "keccak256",
	})

	return cre.ThenPromise(promise, func(report *cre.Report) cre.Promise[*evm.WriteReportReply] {
	    return c.client.WriteReport(runtime, &evm.WriteCreReportRequest{
    		Receiver: c.Address.Bytes(),
    		Report: report,
    		GasConfig: gasConfig,
    	})
	})
}
{{end}}

func (c {{$contract.Type}}) WriteReport(
	runtime cre.Runtime,
	report *cre.Report,
	gasConfig *evm.GasConfig,
) cre.Promise[*evm.WriteReportReply] {
	return c.client.WriteReport(runtime, &evm.WriteCreReportRequest{
		Receiver: c.Address.Bytes(),
		Report: report,
		GasConfig: gasConfig,
	})
}

{{range $error := $contract.Errors}}

// Decode{{.Normalized.Name}}Error decodes a {{.Original.Name}} error from revert data.
func (c *{{$contract.Type}}) Decode{{.Normalized.Name}}Error(data []byte) (*{{.Normalized.Name}}, error) {
	args := c.ABI.Errors["{{.Original.Name}}"].Inputs
	values, err := args.Unpack(data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack error: %w", err)
	}
	if len(values) != {{len .Normalized.Inputs}} {
		return nil, fmt.Errorf("expected {{len .Normalized.Inputs}} values, got %d", len(values))
	}

	{{$err := .}} {{/* capture outer context */}}

	{{range $i, $param := $err.Normalized.Inputs}}
	{{$param.Name}}, ok{{$i}} := values[{{$i}}].({{bindtype $param.Type $.Structs}})
	if !ok{{$i}} {
		return nil, fmt.Errorf("unexpected type for {{$param.Name}} in {{$err.Normalized.Name}} error")
	}
	{{end}}

	return &{{$err.Normalized.Name}}{
		{{- range $i, $param := $err.Normalized.Inputs}}
		{{capitalise $param.Name}}: {{$param.Name}},
		{{- end}}
	}, nil
}

// Error implements the error interface for {{.Normalized.Name}}.
func (e *{{.Normalized.Name}}) Error() string {
	return fmt.Sprintf("{{.Normalized.Name}} error:{{range .Normalized.Inputs}} {{.Name}}=%v;{{end}}"{{range .Normalized.Inputs}}, e.{{capitalise .Name}}{{end}})
}

{{end}}

func (c *{{$contract.Type}}) UnpackError(data []byte) (any, error) {
	switch common.Bytes2Hex(data[:4]) {
	{{range $error := $contract.Errors}}case common.Bytes2Hex(c.ABI.Errors["{{$error.Original.Name}}"].ID.Bytes()[:4]):
		return c.Decode{{$error.Normalized.Name}}Error(data)
	{{end}}default:
		return nil, errors.New("unknown error selector")
	}
}

{{range $event := $contract.Events}}

// {{.Normalized.Name}}Trigger wraps the raw log trigger and provides decoded {{.Normalized.Name}}Decoded data
type {{.Normalized.Name}}Trigger struct {
	cre.Trigger[*evm.Log, *evm.Log]  // Embed the raw trigger
	contract *{{$contract.Type}}      // Keep reference for decoding
}

// Adapt method that decodes the log into {{.Normalized.Name}} data
func (t *{{.Normalized.Name}}Trigger) Adapt(l *evm.Log) (*bindings.DecodedLog[{{.Normalized.Name}}Decoded], error) {
	// Decode the log using the contract's codec
	decoded, err := t.contract.Codec.Decode{{.Normalized.Name}}(l)
	if err != nil {
		return nil, fmt.Errorf("failed to decode {{.Normalized.Name}} log: %w", err)
	}
	
	return &bindings.DecodedLog[{{.Normalized.Name}}Decoded]{
		Log:  l,           // Original log
		Data: *decoded,    // Decoded data
	}, nil
}

func (c *{{$contract.Type}}) LogTrigger{{.Normalized.Name}}Log(chainSelector uint64, confidence evm.ConfidenceLevel, filters []{{.Normalized.Name}}Topics) (cre.Trigger[*evm.Log, *bindings.DecodedLog[{{.Normalized.Name}}Decoded]], error) {
	event := c.ABI.Events["{{.Normalized.Name}}"]
	topics, err := c.Codec.Encode{{.Normalized.Name}}Topics(event, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode topics for {{.Normalized.Name}}: %w", err)
	}

	rawTrigger := evm.LogTrigger(chainSelector, &evm.FilterLogTriggerRequest{
		Addresses:  [][]byte{c.Address.Bytes()},
		Topics:     topics,
		Confidence: confidence,
	})

	return &{{.Normalized.Name}}Trigger{
		Trigger: rawTrigger,
		contract: c,
	}, nil
}


func (c *{{$contract.Type}}) FilterLogs{{.Normalized.Name}}(runtime cre.Runtime, options *bindings.FilterOptions) cre.Promise[*evm.FilterLogsReply] {
	if options == nil {
		options = &bindings.FilterOptions{
			ToBlock: options.ToBlock,
		}
	}
	return c.client.FilterLogs(runtime, &evm.FilterLogsRequest{
		FilterQuery: &evm.FilterQuery{
			Addresses: [][]byte{c.Address.Bytes()},
			Topics:    []*evm.Topics{
				{Topic:[][]byte{c.Codec.{{.Normalized.Name}}LogHash()}},
			},			
			BlockHash: options.BlockHash,
			FromBlock: pb.NewBigIntFromInt(options.FromBlock),
			ToBlock:   pb.NewBigIntFromInt(options.ToBlock),
		},
	})
}
{{end}}

{{end}}
