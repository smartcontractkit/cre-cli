// Code generated — DO NOT EDIT.

//go:build !wasip1

package emptybindings

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

// EmptyContractMock is a mock implementation of EmptyContract for testing.
type EmptyContractMock struct {
}

// NewEmptyContractMock creates a new EmptyContractMock for testing.
func NewEmptyContractMock(address common.Address, clientMock *evmmock.ClientCapability) *EmptyContractMock {
	mock := &EmptyContractMock{}

	codec, err := NewCodec()
	if err != nil {
		panic("failed to create codec for mock: " + err.Error())
	}

	abi := codec.(*Codec).abi
	_ = abi

	funcMap := map[string]func([]byte) ([]byte, error){}

	evmmock.AddContractMock(address, clientMock, funcMap, nil)
	return mock
}
