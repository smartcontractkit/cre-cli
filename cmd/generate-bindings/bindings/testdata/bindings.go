// Code generated — DO NOT EDIT.

package bindings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb2 "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm/bindings"
	"github.com/smartcontractkit/cre-sdk-go/cre"
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

var DataStorageMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"requester\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"reason\",\"type\":\"string\"}],\"name\":\"DataNotFound\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"requester\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"reason\",\"type\":\"string\"}],\"name\":\"DataNotFound2\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"caller\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"message\",\"type\":\"string\"}],\"name\":\"AccessLogged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"DataStored\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"indexed\":true,\"internalType\":\"structDataStorage.UserData\",\"name\":\"userData\",\"type\":\"tuple\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"sender\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"},{\"indexed\":true,\"internalType\":\"bytes[]\",\"name\":\"metadataArray\",\"type\":\"bytes[]\"}],\"name\":\"DynamicEvent\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"NoFields\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getMultipleReserves\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"totalMinted\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"totalReserve\",\"type\":\"uint256\"}],\"internalType\":\"structDataStorage.UpdateReserves[]\",\"name\":\"reserves\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getReserves\",\"outputs\":[{\"components\":[{\"internalType\":\"uint256\",\"name\":\"totalMinted\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"totalReserve\",\"type\":\"uint256\"}],\"internalType\":\"structDataStorage.UpdateReserves\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getTupleReserves\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"totalMinted\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"totalReserve\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getValue\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"message\",\"type\":\"string\"}],\"name\":\"logAccess\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"}],\"name\":\"onReport\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"}],\"name\":\"readData\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"name\":\"storeData\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"}],\"internalType\":\"structDataStorage.UserData\",\"name\":\"userData\",\"type\":\"tuple\"}],\"name\":\"storeUserData\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"newValue\",\"type\":\"string\"}],\"name\":\"updateData\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"oldValue\",\"type\":\"string\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x6080604052348015600e575f5ffd5b50610dfe8061001c5f395ff3fe608060405234801561000f575f5ffd5b506004361061009b575f3560e01c806398458c5d1161006357806398458c5d14610145578063b765cb7c14610158578063bddbb0231461016d578063ccf1582714610180578063f5bfa81514610193575f5ffd5b80630902f1ac1461009f57806320965255146100df578063255e0caf146101085780634ece5b4c1461011d578063805f213214610132575b5f5ffd5b6040805180820182525f80825260209182015281518083018352606480825260c89183019182528351908152905191810191909152015b60405180910390f35b6040805180820190915260048152631d195cdd60e21b60208201525b6040516100d691906106b0565b604080516064815260c86020820152016100d6565b61013061012b36600461070d565b6101a6565b005b61013061014036600461070d565b610232565b610130610153366004610777565b6102c9565b61016061036c565b6040516100d691906107ad565b6100fb61017b36600461070d565b610418565b61013061018e366004610804565b610549565b6100fb6101a1366004610842565b610590565b335f90815260208190526040908190209051839183916101c9908890889061089d565b908152602001604051809103902091826101e4929190610944565b50336001600160a01b03167fc95c7d5d3ac582f659cd004afbea77723e1315567b6557f3c059e8eb9586518f858585856040516102249493929190610a25565b60405180910390a250505050565b5f61023f82840184610adf565b602080820151335f90815291829052604091829020835192519394509092909161026891610b90565b908152602001604051809103902090816102829190610ba6565b508051602082015160405133927fc95c7d5d3ac582f659cd004afbea77723e1315567b6557f3c059e8eb9586518f926102ba92610c60565b60405180910390a25050505050565b6102d66020820182610c8d565b335f9081526020819052604090206102ee8480610c8d565b6040516102fc92919061089d565b90815260200160405180910390209182610317929190610944565b50337fc95c7d5d3ac582f659cd004afbea77723e1315567b6557f3c059e8eb9586518f6103448380610c8d565b6103516020860186610c8d565b6040516103619493929190610a25565b60405180910390a250565b6040805160028082526060828101909352816020015b604080518082019091525f808252602082015281526020019060019003908161038257905050905060405180604001604052806064815260200160c8815250815f815181106103d3576103d3610ccf565b6020026020010181905250604051806040016040528061012c81526020016101908152508160018151811061040a5761040a610ccf565b602002602001018190525090565b335f908152602081905260409081902090516060919061043b908790879061089d565b90815260200160405180910390208054610454906108c0565b80601f0160208091040260200160405190810160405280929190818152602001828054610480906108c0565b80156104cb5780601f106104a2576101008083540402835291602001916104cb565b820191905f5260205f20905b8154815290600101906020018083116104ae57829003601f168201915b5050505050905080515f036105025733858560405163f1e5020960e01b81526004016104f993929190610ce3565b60405180910390fd5b335f9081526020819052604090819020905184918491610525908990899061089d565b90815260200160405180910390209182610540929190610944565b50949350505050565b336001600160a01b03167fe2ab1536af9681ad9e5927bca61830526c4cd932e970162eef77328af1fdcfb58383604051610584929190610d46565b60405180910390a25050565b6001600160a01b0383165f90815260208190526040808220905160609291906105bc908690869061089d565b908152602001604051809103902080546105d5906108c0565b80601f0160208091040260200160405190810160405280929190818152602001828054610601906108c0565b801561064c5780601f106106235761010080835404028352916020019161064c565b820191905f5260205f20905b81548152906001019060200180831161062f57829003601f168201915b5050505050905080515f0361067a5784848460405163f1e5020960e01b81526004016104f993929190610d59565b949350505050565b5f81518084528060208401602086015e5f602082860101526020601f19601f83011685010191505092915050565b602081525f6106c26020830184610682565b9392505050565b5f5f83601f8401126106d9575f5ffd5b5081356001600160401b038111156106ef575f5ffd5b602083019150836020828501011115610706575f5ffd5b9250929050565b5f5f5f5f60408587031215610720575f5ffd5b84356001600160401b03811115610735575f5ffd5b610741878288016106c9565b90955093505060208501356001600160401b0381111561075f575f5ffd5b61076b878288016106c9565b95989497509550505050565b5f60208284031215610787575f5ffd5b81356001600160401b0381111561079c575f5ffd5b8201604081850312156106c2575f5ffd5b602080825282518282018190525f918401906040840190835b818110156107f9576107e383855180518252602090810151910152565b60209390930192604092909201916001016107c6565b509095945050505050565b5f5f60208385031215610815575f5ffd5b82356001600160401b0381111561082a575f5ffd5b610836858286016106c9565b90969095509350505050565b5f5f5f60408486031215610854575f5ffd5b83356001600160a01b038116811461086a575f5ffd5b925060208401356001600160401b03811115610884575f5ffd5b610890868287016106c9565b9497909650939450505050565b818382375f9101908152919050565b634e487b7160e01b5f52604160045260245ffd5b600181811c908216806108d457607f821691505b6020821081036108f257634e487b7160e01b5f52602260045260245ffd5b50919050565b601f82111561093f57805f5260205f20601f840160051c8101602085101561091d5750805b601f840160051c820191505b8181101561093c575f8155600101610929565b50505b505050565b6001600160401b0383111561095b5761095b6108ac565b61096f8361096983546108c0565b836108f8565b5f601f8411600181146109a0575f85156109895750838201355b5f19600387901b1c1916600186901b17835561093c565b5f83815260208120601f198716915b828110156109cf57868501358255602094850194600190920191016109af565b50868210156109eb575f1960f88860031b161c19848701351681555b505060018560011b0183555050505050565b81835281816020850137505f828201602090810191909152601f909101601f19169091010190565b604081525f610a386040830186886109fd565b8281036020840152610a4b8185876109fd565b979650505050505050565b5f82601f830112610a65575f5ffd5b81356001600160401b03811115610a7e57610a7e6108ac565b604051601f8201601f19908116603f011681016001600160401b0381118282101715610aac57610aac6108ac565b604052818152838201602001851015610ac3575f5ffd5b816020850160208301375f918101602001919091529392505050565b5f60208284031215610aef575f5ffd5b81356001600160401b03811115610b04575f5ffd5b820160408185031215610b15575f5ffd5b604080519081016001600160401b0381118282101715610b3757610b376108ac565b60405281356001600160401b03811115610b4f575f5ffd5b610b5b86828501610a56565b82525060208201356001600160401b03811115610b76575f5ffd5b610b8286828501610a56565b602083015250949350505050565b5f82518060208501845e5f920191825250919050565b81516001600160401b03811115610bbf57610bbf6108ac565b610bd381610bcd84546108c0565b846108f8565b6020601f821160018114610c05575f8315610bee5750848201515b5f19600385901b1c1916600184901b17845561093c565b5f84815260208120601f198516915b82811015610c345787850151825560209485019460019092019101610c14565b5084821015610c5157868401515f19600387901b60f8161c191681555b50505050600190811b01905550565b604081525f610c726040830185610682565b8281036020840152610c848185610682565b95945050505050565b5f5f8335601e19843603018112610ca2575f5ffd5b8301803591506001600160401b03821115610cbb575f5ffd5b602001915036819003821315610706575f5ffd5b634e487b7160e01b5f52603260045260245ffd5b6001600160a01b03841681526060602082018190525f90610d0790830184866109fd565b828103604093840152601a81527f4e6f206578697374696e67206461746120746f20757064617465000000000000602082015291909101949350505050565b602081525f61067a6020830184866109fd565b6001600160a01b03841681526060602082018190525f90610d7d90830184866109fd565b8281036040840152602181527f4e6f2064617461206173736f63696174656420776974682074686973206b65796020820152601760f91b60408201526060810191505094935050505056fea26469706673582212206938aba98b7e3f58f5746f55b3e72f9e14ee3ad529e8ff28f32829e2cc0303c364736f6c634300081e0033",
}

// Structs
type UpdateReserves struct {
	TotalMinted  *big.Int
	TotalReserve *big.Int
}

type UserData struct {
	Key   string
	Value string
}

// Contract Method Inputs
type LogAccessInput struct {
	Message string
}

type OnReportInput struct {
	Metadata []byte
	Payload  []byte
}

type ReadDataInput struct {
	User common.Address
	Key  string
}

type StoreDataInput struct {
	Key   string
	Value string
}

type StoreUserDataInput struct {
	UserData UserData
}

type UpdateDataInput struct {
	Key      string
	NewValue string
}

// Contract Method Outputs
type GetTupleReservesOutput struct {
	TotalMinted  *big.Int
	TotalReserve *big.Int
}

// Errors
type DataNotFound struct {
	Requester common.Address
	Key       string
	Reason    string
}

type DataNotFound2 struct {
	Requester common.Address
	Key       string
	Reason    string
}

// Events
// The <Event> struct should be used as a filter (for log triggers).
// Indexed (string and bytes) fields will be of type common.Hash.
// They need to he (crypto.Keccak256) hashed and passed in.
// Indexed (tuple/slice/array) fields can be passed in as is, the Encode<Event>Topics function will handle the hashing.
//
// The <Event>Decoded struct will be the result of calling decode (Adapt) on the log trigger result.
// Indexed dynamic type fields will be of type common.Hash.

type AccessLogged struct {
	Caller  common.Address
	Message string
}

type AccessLoggedDecoded struct {
	Caller  common.Address
	Message string
}

type DataStored struct {
	Sender common.Address
	Key    string
	Value  string
}

type DataStoredDecoded struct {
	Sender common.Address
	Key    string
	Value  string
}

type DynamicEvent struct {
	Key           string
	UserData      UserData
	Sender        string
	Metadata      common.Hash
	MetadataArray [][]byte
}

type DynamicEventDecoded struct {
	Key           string
	UserData      common.Hash
	Sender        string
	Metadata      common.Hash
	MetadataArray common.Hash
}

type NoFields struct {
}

type NoFieldsDecoded struct {
}

// Main Binding Type for DataStorage
type DataStorage struct {
	Address common.Address
	Options *bindings.ContractInitOptions
	ABI     *abi.ABI
	client  *evm.Client
	Codec   DataStorageCodec
}

type DataStorageCodec interface {
	EncodeGetMultipleReservesMethodCall() ([]byte, error)
	DecodeGetMultipleReservesMethodOutput(data []byte) ([]UpdateReserves, error)
	EncodeGetReservesMethodCall() ([]byte, error)
	DecodeGetReservesMethodOutput(data []byte) (UpdateReserves, error)
	EncodeGetTupleReservesMethodCall() ([]byte, error)
	DecodeGetTupleReservesMethodOutput(data []byte) (GetTupleReservesOutput, error)
	EncodeGetValueMethodCall() ([]byte, error)
	DecodeGetValueMethodOutput(data []byte) (string, error)
	EncodeLogAccessMethodCall(in LogAccessInput) ([]byte, error)
	EncodeOnReportMethodCall(in OnReportInput) ([]byte, error)
	EncodeReadDataMethodCall(in ReadDataInput) ([]byte, error)
	DecodeReadDataMethodOutput(data []byte) (string, error)
	EncodeStoreDataMethodCall(in StoreDataInput) ([]byte, error)
	EncodeStoreUserDataMethodCall(in StoreUserDataInput) ([]byte, error)
	EncodeUpdateDataMethodCall(in UpdateDataInput) ([]byte, error)
	DecodeUpdateDataMethodOutput(data []byte) (string, error)
	EncodeUpdateReservesStruct(in UpdateReserves) ([]byte, error)
	EncodeUserDataStruct(in UserData) ([]byte, error)
	AccessLoggedLogHash() []byte
	EncodeAccessLoggedTopics(evt abi.Event, values []AccessLogged) ([]*evm.TopicValues, error)
	DecodeAccessLogged(log *evm.Log) (*AccessLoggedDecoded, error)
	DataStoredLogHash() []byte
	EncodeDataStoredTopics(evt abi.Event, values []DataStored) ([]*evm.TopicValues, error)
	DecodeDataStored(log *evm.Log) (*DataStoredDecoded, error)
	DynamicEventLogHash() []byte
	EncodeDynamicEventTopics(evt abi.Event, values []DynamicEvent) ([]*evm.TopicValues, error)
	DecodeDynamicEvent(log *evm.Log) (*DynamicEventDecoded, error)
	NoFieldsLogHash() []byte
	EncodeNoFieldsTopics(evt abi.Event, values []NoFields) ([]*evm.TopicValues, error)
	DecodeNoFields(log *evm.Log) (*NoFieldsDecoded, error)
}

func NewDataStorage(
	client *evm.Client,
	address common.Address,
	options *bindings.ContractInitOptions,
) (*DataStorage, error) {
	parsed, err := abi.JSON(strings.NewReader(DataStorageMetaData.ABI))
	if err != nil {
		return nil, err
	}
	codec, err := NewCodec()
	if err != nil {
		return nil, err
	}
	return &DataStorage{
		Address: address,
		Options: options,
		ABI:     &parsed,
		client:  client,
		Codec:   codec,
	}, nil
}

type Codec struct {
	abi *abi.ABI
}

func NewCodec() (DataStorageCodec, error) {
	parsed, err := abi.JSON(strings.NewReader(DataStorageMetaData.ABI))
	if err != nil {
		return nil, err
	}
	return &Codec{abi: &parsed}, nil
}

func (c *Codec) EncodeGetMultipleReservesMethodCall() ([]byte, error) {
	return c.abi.Pack("getMultipleReserves")
}

func (c *Codec) DecodeGetMultipleReservesMethodOutput(data []byte) ([]UpdateReserves, error) {
	vals, err := c.abi.Methods["getMultipleReserves"].Outputs.Unpack(data)
	if err != nil {
		return *new([]UpdateReserves), err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return *new([]UpdateReserves), fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result []UpdateReserves
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return *new([]UpdateReserves), fmt.Errorf("failed to unmarshal to []UpdateReserves: %w", err)
	}

	return result, nil
}

func (c *Codec) EncodeGetReservesMethodCall() ([]byte, error) {
	return c.abi.Pack("getReserves")
}

func (c *Codec) DecodeGetReservesMethodOutput(data []byte) (UpdateReserves, error) {
	vals, err := c.abi.Methods["getReserves"].Outputs.Unpack(data)
	if err != nil {
		return *new(UpdateReserves), err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return *new(UpdateReserves), fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result UpdateReserves
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return *new(UpdateReserves), fmt.Errorf("failed to unmarshal to UpdateReserves: %w", err)
	}

	return result, nil
}

func (c *Codec) EncodeGetTupleReservesMethodCall() ([]byte, error) {
	return c.abi.Pack("getTupleReserves")
}

func (c *Codec) DecodeGetTupleReservesMethodOutput(data []byte) (GetTupleReservesOutput, error) {
	vals, err := c.abi.Methods["getTupleReserves"].Outputs.Unpack(data)
	if err != nil {
		return GetTupleReservesOutput{}, err
	}
	if len(vals) != 2 {
		return GetTupleReservesOutput{}, fmt.Errorf("expected 2 values, got %d", len(vals))
	}
	jsonData0, err := json.Marshal(vals[0])
	if err != nil {
		return GetTupleReservesOutput{}, fmt.Errorf("failed to marshal ABI result 0: %w", err)
	}

	var result0 *big.Int
	if err := json.Unmarshal(jsonData0, &result0); err != nil {
		return GetTupleReservesOutput{}, fmt.Errorf("failed to unmarshal to *big.Int: %w", err)
	}
	jsonData1, err := json.Marshal(vals[1])
	if err != nil {
		return GetTupleReservesOutput{}, fmt.Errorf("failed to marshal ABI result 1: %w", err)
	}

	var result1 *big.Int
	if err := json.Unmarshal(jsonData1, &result1); err != nil {
		return GetTupleReservesOutput{}, fmt.Errorf("failed to unmarshal to *big.Int: %w", err)
	}

	return GetTupleReservesOutput{
		TotalMinted:  result0,
		TotalReserve: result1,
	}, nil
}

func (c *Codec) EncodeGetValueMethodCall() ([]byte, error) {
	return c.abi.Pack("getValue")
}

func (c *Codec) DecodeGetValueMethodOutput(data []byte) (string, error) {
	vals, err := c.abi.Methods["getValue"].Outputs.Unpack(data)
	if err != nil {
		return *new(string), err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return *new(string), fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result string
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return *new(string), fmt.Errorf("failed to unmarshal to string: %w", err)
	}

	return result, nil
}

func (c *Codec) EncodeLogAccessMethodCall(in LogAccessInput) ([]byte, error) {
	return c.abi.Pack("logAccess", in.Message)
}

func (c *Codec) EncodeOnReportMethodCall(in OnReportInput) ([]byte, error) {
	return c.abi.Pack("onReport", in.Metadata, in.Payload)
}

func (c *Codec) EncodeReadDataMethodCall(in ReadDataInput) ([]byte, error) {
	return c.abi.Pack("readData", in.User, in.Key)
}

func (c *Codec) DecodeReadDataMethodOutput(data []byte) (string, error) {
	vals, err := c.abi.Methods["readData"].Outputs.Unpack(data)
	if err != nil {
		return *new(string), err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return *new(string), fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result string
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return *new(string), fmt.Errorf("failed to unmarshal to string: %w", err)
	}

	return result, nil
}

func (c *Codec) EncodeStoreDataMethodCall(in StoreDataInput) ([]byte, error) {
	return c.abi.Pack("storeData", in.Key, in.Value)
}

func (c *Codec) EncodeStoreUserDataMethodCall(in StoreUserDataInput) ([]byte, error) {
	return c.abi.Pack("storeUserData", in.UserData)
}

func (c *Codec) EncodeUpdateDataMethodCall(in UpdateDataInput) ([]byte, error) {
	return c.abi.Pack("updateData", in.Key, in.NewValue)
}

func (c *Codec) DecodeUpdateDataMethodOutput(data []byte) (string, error) {
	vals, err := c.abi.Methods["updateData"].Outputs.Unpack(data)
	if err != nil {
		return *new(string), err
	}
	jsonData, err := json.Marshal(vals[0])
	if err != nil {
		return *new(string), fmt.Errorf("failed to marshal ABI result: %w", err)
	}

	var result string
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return *new(string), fmt.Errorf("failed to unmarshal to string: %w", err)
	}

	return result, nil
}

func (c *Codec) EncodeUpdateReservesStruct(in UpdateReserves) ([]byte, error) {
	tupleType, err := abi.NewType(
		"tuple", "",
		[]abi.ArgumentMarshaling{
			{Name: "totalMinted", Type: "uint256"},
			{Name: "totalReserve", Type: "uint256"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tuple type for UpdateReserves: %w", err)
	}
	args := abi.Arguments{
		{Name: "updateReserves", Type: tupleType},
	}

	return args.Pack(in)
}
func (c *Codec) EncodeUserDataStruct(in UserData) ([]byte, error) {
	tupleType, err := abi.NewType(
		"tuple", "",
		[]abi.ArgumentMarshaling{
			{Name: "key", Type: "string"},
			{Name: "value", Type: "string"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tuple type for UserData: %w", err)
	}
	args := abi.Arguments{
		{Name: "userData", Type: tupleType},
	}

	return args.Pack(in)
}

func (c *Codec) AccessLoggedLogHash() []byte {
	return c.abi.Events["AccessLogged"].ID.Bytes()
}

func (c *Codec) EncodeAccessLoggedTopics(
	evt abi.Event,
	values []AccessLogged,
) ([]*evm.TopicValues, error) {
	var callerRule []interface{}
	for _, v := range values {
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[0], v.Caller)
		if err != nil {
			return nil, err
		}
		callerRule = append(callerRule, fieldVal)
	}

	rawTopics, err := abi.MakeTopics(
		callerRule,
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
			bs[j] = h.Bytes()
		}
		topics[i+1] = &evm.TopicValues{Values: bs}
	}
	return topics, nil
}

// DecodeAccessLogged decodes a log into a AccessLogged struct.
func (c *Codec) DecodeAccessLogged(log *evm.Log) (*AccessLoggedDecoded, error) {
	event := new(AccessLoggedDecoded)
	if err := c.abi.UnpackIntoInterface(event, "AccessLogged", log.Data); err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events["AccessLogged"].Inputs {
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

func (c *Codec) DataStoredLogHash() []byte {
	return c.abi.Events["DataStored"].ID.Bytes()
}

func (c *Codec) EncodeDataStoredTopics(
	evt abi.Event,
	values []DataStored,
) ([]*evm.TopicValues, error) {
	var senderRule []interface{}
	for _, v := range values {
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[0], v.Sender)
		if err != nil {
			return nil, err
		}
		senderRule = append(senderRule, fieldVal)
	}

	rawTopics, err := abi.MakeTopics(
		senderRule,
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
			bs[j] = h.Bytes()
		}
		topics[i+1] = &evm.TopicValues{Values: bs}
	}
	return topics, nil
}

// DecodeDataStored decodes a log into a DataStored struct.
func (c *Codec) DecodeDataStored(log *evm.Log) (*DataStoredDecoded, error) {
	event := new(DataStoredDecoded)
	if err := c.abi.UnpackIntoInterface(event, "DataStored", log.Data); err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events["DataStored"].Inputs {
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

func (c *Codec) DynamicEventLogHash() []byte {
	return c.abi.Events["DynamicEvent"].ID.Bytes()
}

func (c *Codec) EncodeDynamicEventTopics(
	evt abi.Event,
	values []DynamicEvent,
) ([]*evm.TopicValues, error) {
	var userDataRule []interface{}
	for _, v := range values {
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[1], v.UserData)
		if err != nil {
			return nil, err
		}
		userDataRule = append(userDataRule, fieldVal)
	}
	var metadataRule []interface{}
	for _, v := range values {
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[3], v.Metadata)
		if err != nil {
			return nil, err
		}
		metadataRule = append(metadataRule, fieldVal)
	}
	var metadataArrayRule []interface{}
	for _, v := range values {
		fieldVal, err := bindings.PrepareTopicArg(evt.Inputs[4], v.MetadataArray)
		if err != nil {
			return nil, err
		}
		metadataArrayRule = append(metadataArrayRule, fieldVal)
	}

	rawTopics, err := abi.MakeTopics(
		userDataRule,
		metadataRule,
		metadataArrayRule,
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
			bs[j] = h.Bytes()
		}
		topics[i+1] = &evm.TopicValues{Values: bs}
	}
	return topics, nil
}

// DecodeDynamicEvent decodes a log into a DynamicEvent struct.
func (c *Codec) DecodeDynamicEvent(log *evm.Log) (*DynamicEventDecoded, error) {
	event := new(DynamicEventDecoded)
	if err := c.abi.UnpackIntoInterface(event, "DynamicEvent", log.Data); err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events["DynamicEvent"].Inputs {
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

func (c *Codec) NoFieldsLogHash() []byte {
	return c.abi.Events["NoFields"].ID.Bytes()
}

func (c *Codec) EncodeNoFieldsTopics(
	evt abi.Event,
	values []NoFields,
) ([]*evm.TopicValues, error) {

	rawTopics, err := abi.MakeTopics()
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
			bs[j] = h.Bytes()
		}
		topics[i+1] = &evm.TopicValues{Values: bs}
	}
	return topics, nil
}

// DecodeNoFields decodes a log into a NoFields struct.
func (c *Codec) DecodeNoFields(log *evm.Log) (*NoFieldsDecoded, error) {
	event := new(NoFieldsDecoded)
	if err := c.abi.UnpackIntoInterface(event, "NoFields", log.Data); err != nil {
		return nil, err
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events["NoFields"].Inputs {
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

func (c DataStorage) GetMultipleReserves(
	runtime cre.Runtime,
	blockNumber *big.Int,
) cre.Promise[[]UpdateReserves] {
	calldata, err := c.Codec.EncodeGetMultipleReservesMethodCall()
	if err != nil {
		return cre.PromiseFromResult[[]UpdateReserves](*new([]UpdateReserves), err)
	}

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: pb.NewBigIntFromInt(big.NewInt(rpc.FinalizedBlockNumber.Int64())),
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
	return cre.Then(promise, func(response *evm.CallContractReply) ([]UpdateReserves, error) {
		return c.Codec.DecodeGetMultipleReservesMethodOutput(response.Data)
	})

}

func (c DataStorage) GetReserves(
	runtime cre.Runtime,
	blockNumber *big.Int,
) cre.Promise[UpdateReserves] {
	calldata, err := c.Codec.EncodeGetReservesMethodCall()
	if err != nil {
		return cre.PromiseFromResult[UpdateReserves](*new(UpdateReserves), err)
	}

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: pb.NewBigIntFromInt(big.NewInt(rpc.FinalizedBlockNumber.Int64())),
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
	return cre.Then(promise, func(response *evm.CallContractReply) (UpdateReserves, error) {
		return c.Codec.DecodeGetReservesMethodOutput(response.Data)
	})

}

func (c DataStorage) GetTupleReserves(
	runtime cre.Runtime,
	blockNumber *big.Int,
) cre.Promise[GetTupleReservesOutput] {
	calldata, err := c.Codec.EncodeGetTupleReservesMethodCall()
	if err != nil {
		return cre.PromiseFromResult[GetTupleReservesOutput](GetTupleReservesOutput{}, err)
	}

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: pb.NewBigIntFromInt(big.NewInt(rpc.FinalizedBlockNumber.Int64())),
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
	return cre.Then(promise, func(response *evm.CallContractReply) (GetTupleReservesOutput, error) {
		return c.Codec.DecodeGetTupleReservesMethodOutput(response.Data)
	})

}

func (c DataStorage) GetValue(
	runtime cre.Runtime,
	blockNumber *big.Int,
) cre.Promise[string] {
	calldata, err := c.Codec.EncodeGetValueMethodCall()
	if err != nil {
		return cre.PromiseFromResult[string](*new(string), err)
	}

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: pb.NewBigIntFromInt(big.NewInt(rpc.FinalizedBlockNumber.Int64())),
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
	return cre.Then(promise, func(response *evm.CallContractReply) (string, error) {
		return c.Codec.DecodeGetValueMethodOutput(response.Data)
	})

}

func (c DataStorage) ReadData(
	runtime cre.Runtime,
	args ReadDataInput,
	blockNumber *big.Int,
) cre.Promise[string] {
	calldata, err := c.Codec.EncodeReadDataMethodCall(args)
	if err != nil {
		return cre.PromiseFromResult[string](*new(string), err)
	}

	var bn cre.Promise[*pb.BigInt]
	if blockNumber == nil {
		promise := c.client.HeaderByNumber(runtime, &evm.HeaderByNumberRequest{
			BlockNumber: pb.NewBigIntFromInt(big.NewInt(rpc.FinalizedBlockNumber.Int64())),
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
	return cre.Then(promise, func(response *evm.CallContractReply) (string, error) {
		return c.Codec.DecodeReadDataMethodOutput(response.Data)
	})

}

func (c DataStorage) WriteReportFromUpdateReserves(
	runtime cre.Runtime,
	input UpdateReserves,
	gasConfig *evm.GasConfig,
) cre.Promise[*evm.WriteReportReply] {
	encoded, err := c.Codec.EncodeUpdateReservesStruct(input)
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
			Receiver:  c.Address.Bytes(),
			Report:    report,
			GasConfig: gasConfig,
		})
	})
}

func (c DataStorage) WriteReportFromUserData(
	runtime cre.Runtime,
	input UserData,
	gasConfig *evm.GasConfig,
) cre.Promise[*evm.WriteReportReply] {
	encoded, err := c.Codec.EncodeUserDataStruct(input)
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
			Receiver:  c.Address.Bytes(),
			Report:    report,
			GasConfig: gasConfig,
		})
	})
}

func (c DataStorage) WriteReport(
	runtime cre.Runtime,
	report *cre.Report,
	gasConfig *evm.GasConfig,
) cre.Promise[*evm.WriteReportReply] {
	return c.client.WriteReport(runtime, &evm.WriteCreReportRequest{
		Receiver:  c.Address.Bytes(),
		Report:    report,
		GasConfig: gasConfig,
	})
}

// DecodeDataNotFoundError decodes a DataNotFound error from revert data.
func (c *DataStorage) DecodeDataNotFoundError(data []byte) (*DataNotFound, error) {
	args := c.ABI.Errors["DataNotFound"].Inputs
	values, err := args.Unpack(data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack error: %w", err)
	}
	if len(values) != 3 {
		return nil, fmt.Errorf("expected 3 values, got %d", len(values))
	}

	requester, ok0 := values[0].(common.Address)
	if !ok0 {
		return nil, fmt.Errorf("unexpected type for requester in DataNotFound error")
	}

	key, ok1 := values[1].(string)
	if !ok1 {
		return nil, fmt.Errorf("unexpected type for key in DataNotFound error")
	}

	reason, ok2 := values[2].(string)
	if !ok2 {
		return nil, fmt.Errorf("unexpected type for reason in DataNotFound error")
	}

	return &DataNotFound{
		Requester: requester,
		Key:       key,
		Reason:    reason,
	}, nil
}

// Error implements the error interface for DataNotFound.
func (e *DataNotFound) Error() string {
	return fmt.Sprintf("DataNotFound error: requester=%v; key=%v; reason=%v;", e.Requester, e.Key, e.Reason)
}

// DecodeDataNotFound2Error decodes a DataNotFound2 error from revert data.
func (c *DataStorage) DecodeDataNotFound2Error(data []byte) (*DataNotFound2, error) {
	args := c.ABI.Errors["DataNotFound2"].Inputs
	values, err := args.Unpack(data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack error: %w", err)
	}
	if len(values) != 3 {
		return nil, fmt.Errorf("expected 3 values, got %d", len(values))
	}

	requester, ok0 := values[0].(common.Address)
	if !ok0 {
		return nil, fmt.Errorf("unexpected type for requester in DataNotFound2 error")
	}

	key, ok1 := values[1].(string)
	if !ok1 {
		return nil, fmt.Errorf("unexpected type for key in DataNotFound2 error")
	}

	reason, ok2 := values[2].(string)
	if !ok2 {
		return nil, fmt.Errorf("unexpected type for reason in DataNotFound2 error")
	}

	return &DataNotFound2{
		Requester: requester,
		Key:       key,
		Reason:    reason,
	}, nil
}

// Error implements the error interface for DataNotFound2.
func (e *DataNotFound2) Error() string {
	return fmt.Sprintf("DataNotFound2 error: requester=%v; key=%v; reason=%v;", e.Requester, e.Key, e.Reason)
}

func (c *DataStorage) UnpackError(data []byte) (any, error) {
	switch common.Bytes2Hex(data[:4]) {
	case common.Bytes2Hex(c.ABI.Errors["DataNotFound"].ID.Bytes()[:4]):
		return c.DecodeDataNotFoundError(data)
	case common.Bytes2Hex(c.ABI.Errors["DataNotFound2"].ID.Bytes()[:4]):
		return c.DecodeDataNotFound2Error(data)
	default:
		return nil, errors.New("unknown error selector")
	}
}

// AccessLoggedTrigger wraps the raw log trigger and provides decoded AccessLoggedDecoded data
type AccessLoggedTrigger struct {
	cre.Trigger[*evm.Log, *evm.Log]              // Embed the raw trigger
	contract                        *DataStorage // Keep reference for decoding
}

// Adapt method that decodes the log into AccessLogged data
func (t *AccessLoggedTrigger) Adapt(l *evm.Log) (*bindings.DecodedLog[AccessLoggedDecoded], error) {
	// Decode the log using the contract's codec
	decoded, err := t.contract.Codec.DecodeAccessLogged(l)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AccessLogged log: %w", err)
	}

	return &bindings.DecodedLog[AccessLoggedDecoded]{
		Log:  l,        // Original log
		Data: *decoded, // Decoded data
	}, nil
}

func (c *DataStorage) LogTriggerAccessLoggedLog(chainSelector uint64, confidence evm.ConfidenceLevel, filters []AccessLogged) (cre.Trigger[*evm.Log, *bindings.DecodedLog[AccessLoggedDecoded]], error) {
	event := c.ABI.Events["AccessLogged"]
	topics, err := c.Codec.EncodeAccessLoggedTopics(event, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode topics for AccessLogged: %w", err)
	}

	rawTrigger := evm.LogTrigger(chainSelector, &evm.FilterLogTriggerRequest{
		Addresses:  [][]byte{c.Address.Bytes()},
		Topics:     topics,
		Confidence: confidence,
	})

	return &AccessLoggedTrigger{
		Trigger:  rawTrigger,
		contract: c,
	}, nil
}

func (c *DataStorage) FilterLogsAccessLogged(runtime cre.Runtime, options *bindings.FilterOptions) cre.Promise[*evm.FilterLogsReply] {
	if options == nil {
		options = &bindings.FilterOptions{
			ToBlock: options.ToBlock,
		}
	}
	return c.client.FilterLogs(runtime, &evm.FilterLogsRequest{
		FilterQuery: &evm.FilterQuery{
			Addresses: [][]byte{c.Address.Bytes()},
			Topics: []*evm.Topics{
				{Topic: [][]byte{c.Codec.AccessLoggedLogHash()}},
			},
			BlockHash: options.BlockHash,
			FromBlock: pb.NewBigIntFromInt(options.FromBlock),
			ToBlock:   pb.NewBigIntFromInt(options.ToBlock),
		},
	})
}

// DataStoredTrigger wraps the raw log trigger and provides decoded DataStoredDecoded data
type DataStoredTrigger struct {
	cre.Trigger[*evm.Log, *evm.Log]              // Embed the raw trigger
	contract                        *DataStorage // Keep reference for decoding
}

// Adapt method that decodes the log into DataStored data
func (t *DataStoredTrigger) Adapt(l *evm.Log) (*bindings.DecodedLog[DataStoredDecoded], error) {
	// Decode the log using the contract's codec
	decoded, err := t.contract.Codec.DecodeDataStored(l)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DataStored log: %w", err)
	}

	return &bindings.DecodedLog[DataStoredDecoded]{
		Log:  l,        // Original log
		Data: *decoded, // Decoded data
	}, nil
}

func (c *DataStorage) LogTriggerDataStoredLog(chainSelector uint64, confidence evm.ConfidenceLevel, filters []DataStored) (cre.Trigger[*evm.Log, *bindings.DecodedLog[DataStoredDecoded]], error) {
	event := c.ABI.Events["DataStored"]
	topics, err := c.Codec.EncodeDataStoredTopics(event, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode topics for DataStored: %w", err)
	}

	rawTrigger := evm.LogTrigger(chainSelector, &evm.FilterLogTriggerRequest{
		Addresses:  [][]byte{c.Address.Bytes()},
		Topics:     topics,
		Confidence: confidence,
	})

	return &DataStoredTrigger{
		Trigger:  rawTrigger,
		contract: c,
	}, nil
}

func (c *DataStorage) FilterLogsDataStored(runtime cre.Runtime, options *bindings.FilterOptions) cre.Promise[*evm.FilterLogsReply] {
	if options == nil {
		options = &bindings.FilterOptions{
			ToBlock: options.ToBlock,
		}
	}
	return c.client.FilterLogs(runtime, &evm.FilterLogsRequest{
		FilterQuery: &evm.FilterQuery{
			Addresses: [][]byte{c.Address.Bytes()},
			Topics: []*evm.Topics{
				{Topic: [][]byte{c.Codec.DataStoredLogHash()}},
			},
			BlockHash: options.BlockHash,
			FromBlock: pb.NewBigIntFromInt(options.FromBlock),
			ToBlock:   pb.NewBigIntFromInt(options.ToBlock),
		},
	})
}

// DynamicEventTrigger wraps the raw log trigger and provides decoded DynamicEventDecoded data
type DynamicEventTrigger struct {
	cre.Trigger[*evm.Log, *evm.Log]              // Embed the raw trigger
	contract                        *DataStorage // Keep reference for decoding
}

// Adapt method that decodes the log into DynamicEvent data
func (t *DynamicEventTrigger) Adapt(l *evm.Log) (*bindings.DecodedLog[DynamicEventDecoded], error) {
	// Decode the log using the contract's codec
	decoded, err := t.contract.Codec.DecodeDynamicEvent(l)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DynamicEvent log: %w", err)
	}

	return &bindings.DecodedLog[DynamicEventDecoded]{
		Log:  l,        // Original log
		Data: *decoded, // Decoded data
	}, nil
}

func (c *DataStorage) LogTriggerDynamicEventLog(chainSelector uint64, confidence evm.ConfidenceLevel, filters []DynamicEvent) (cre.Trigger[*evm.Log, *bindings.DecodedLog[DynamicEventDecoded]], error) {
	event := c.ABI.Events["DynamicEvent"]
	topics, err := c.Codec.EncodeDynamicEventTopics(event, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode topics for DynamicEvent: %w", err)
	}

	rawTrigger := evm.LogTrigger(chainSelector, &evm.FilterLogTriggerRequest{
		Addresses:  [][]byte{c.Address.Bytes()},
		Topics:     topics,
		Confidence: confidence,
	})

	return &DynamicEventTrigger{
		Trigger:  rawTrigger,
		contract: c,
	}, nil
}

func (c *DataStorage) FilterLogsDynamicEvent(runtime cre.Runtime, options *bindings.FilterOptions) cre.Promise[*evm.FilterLogsReply] {
	if options == nil {
		options = &bindings.FilterOptions{
			ToBlock: options.ToBlock,
		}
	}
	return c.client.FilterLogs(runtime, &evm.FilterLogsRequest{
		FilterQuery: &evm.FilterQuery{
			Addresses: [][]byte{c.Address.Bytes()},
			Topics: []*evm.Topics{
				{Topic: [][]byte{c.Codec.DynamicEventLogHash()}},
			},
			BlockHash: options.BlockHash,
			FromBlock: pb.NewBigIntFromInt(options.FromBlock),
			ToBlock:   pb.NewBigIntFromInt(options.ToBlock),
		},
	})
}

// NoFieldsTrigger wraps the raw log trigger and provides decoded NoFieldsDecoded data
type NoFieldsTrigger struct {
	cre.Trigger[*evm.Log, *evm.Log]              // Embed the raw trigger
	contract                        *DataStorage // Keep reference for decoding
}

// Adapt method that decodes the log into NoFields data
func (t *NoFieldsTrigger) Adapt(l *evm.Log) (*bindings.DecodedLog[NoFieldsDecoded], error) {
	// Decode the log using the contract's codec
	decoded, err := t.contract.Codec.DecodeNoFields(l)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NoFields log: %w", err)
	}

	return &bindings.DecodedLog[NoFieldsDecoded]{
		Log:  l,        // Original log
		Data: *decoded, // Decoded data
	}, nil
}

func (c *DataStorage) LogTriggerNoFieldsLog(chainSelector uint64, confidence evm.ConfidenceLevel, filters []NoFields) (cre.Trigger[*evm.Log, *bindings.DecodedLog[NoFieldsDecoded]], error) {
	event := c.ABI.Events["NoFields"]
	topics, err := c.Codec.EncodeNoFieldsTopics(event, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to encode topics for NoFields: %w", err)
	}

	rawTrigger := evm.LogTrigger(chainSelector, &evm.FilterLogTriggerRequest{
		Addresses:  [][]byte{c.Address.Bytes()},
		Topics:     topics,
		Confidence: confidence,
	})

	return &NoFieldsTrigger{
		Trigger:  rawTrigger,
		contract: c,
	}, nil
}

func (c *DataStorage) FilterLogsNoFields(runtime cre.Runtime, options *bindings.FilterOptions) cre.Promise[*evm.FilterLogsReply] {
	if options == nil {
		options = &bindings.FilterOptions{
			ToBlock: options.ToBlock,
		}
	}
	return c.client.FilterLogs(runtime, &evm.FilterLogsRequest{
		FilterQuery: &evm.FilterQuery{
			Addresses: [][]byte{c.Address.Bytes()},
			Topics: []*evm.Topics{
				{Topic: [][]byte{c.Codec.NoFieldsLogHash()}},
			},
			BlockHash: options.BlockHash,
			FromBlock: pb.NewBigIntFromInt(options.FromBlock),
			ToBlock:   pb.NewBigIntFromInt(options.ToBlock),
		},
	})
}
