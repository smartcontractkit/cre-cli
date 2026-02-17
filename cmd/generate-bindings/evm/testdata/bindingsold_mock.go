// Code generated — DO NOT EDIT.

//go:build !wasip1

package bindingsold

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

// DataStorageMock is a mock implementation of DataStorage for testing.
type DataStorageMock struct {
	GetMultipleReserves func() ([]UpdateReserves, error)
	GetReserves         func() (UpdateReserves, error)
	GetTupleReserves    func() (GetTupleReservesOutput, error)
	GetValue            func() (string, error)
	ReadData            func(ReadDataInput) (string, error)
}

// NewDataStorageMock creates a new DataStorageMock for testing.
func NewDataStorageMock(address common.Address, clientMock *evmmock.ClientCapability) *DataStorageMock {
	mock := &DataStorageMock{}

	codec, err := NewCodec()
	if err != nil {
		panic("failed to create codec for mock: " + err.Error())
	}

	abi := codec.(*Codec).abi
	_ = abi

	funcMap := map[string]func([]byte) ([]byte, error){
		string(abi.Methods["getMultipleReserves"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.GetMultipleReserves == nil {
				return nil, errors.New("getMultipleReserves method not mocked")
			}
			result, err := mock.GetMultipleReserves()
			if err != nil {
				return nil, err
			}
			return abi.Methods["getMultipleReserves"].Outputs.Pack(result)
		},
		string(abi.Methods["getReserves"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.GetReserves == nil {
				return nil, errors.New("getReserves method not mocked")
			}
			result, err := mock.GetReserves()
			if err != nil {
				return nil, err
			}
			return abi.Methods["getReserves"].Outputs.Pack(result)
		},
		string(abi.Methods["getTupleReserves"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.GetTupleReserves == nil {
				return nil, errors.New("getTupleReserves method not mocked")
			}
			result, err := mock.GetTupleReserves()
			if err != nil {
				return nil, err
			}
			return abi.Methods["getTupleReserves"].Outputs.Pack(
				result.TotalMinted,
				result.TotalReserve,
			)
		},
		string(abi.Methods["getValue"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.GetValue == nil {
				return nil, errors.New("getValue method not mocked")
			}
			result, err := mock.GetValue()
			if err != nil {
				return nil, err
			}
			return abi.Methods["getValue"].Outputs.Pack(result)
		},
		string(abi.Methods["readData"].ID[:4]): func(payload []byte) ([]byte, error) {
			if mock.ReadData == nil {
				return nil, errors.New("readData method not mocked")
			}
			inputs := abi.Methods["readData"].Inputs

			values, err := inputs.Unpack(payload)
			if err != nil {
				return nil, errors.New("Failed to unpack payload")
			}
			if len(values) != 2 {
				return nil, errors.New("expected 2 input values")
			}

			args := ReadDataInput{
				User: values[0].(common.Address),
				Key:  values[1].(string),
			}

			result, err := mock.ReadData(args)
			if err != nil {
				return nil, err
			}
			return abi.Methods["readData"].Outputs.Pack(result)
		},
	}

	evmmock.AddContractMock(address, clientMock, funcMap, nil)
	return mock
}
