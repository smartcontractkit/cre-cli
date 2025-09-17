package chainsim

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
)

const (
	TestAddress    = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
	TestPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
)

type SimulatedChain struct {
	Backend *simulated.Backend
}

func NewSimulatedChain() *SimulatedChain {
	startBackend := func(fundedAddresses []common.Address) *simulated.Backend {
		toFund := make(map[common.Address]types.Account)

		for _, address := range fundedAddresses {
			toFund[address] = types.Account{
				Balance: big.NewInt(1000000000000000000), // 1 Ether
			}
		}
		backend := simulated.NewBackend(toFund)
		return backend
	}

	backend := startBackend(
		[]common.Address{common.HexToAddress(TestAddress)},
	)

	return &SimulatedChain{
		Backend: backend,
	}
}

func (sc *SimulatedChain) Close() {
	sc.Backend.Close()
}
