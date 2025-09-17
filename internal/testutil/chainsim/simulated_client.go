package chainsim

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/dev-platform/cmd/client"
)

func NewSimulatedClient(t *testing.T, chain *SimulatedChain, logger *zerolog.Logger) *seth.Client {
	hooks := seth.Hooks{
		ContractDeployment: seth.ContractDeploymentHooks{
			Post: func(sethClient *seth.Client, tx *types.Transaction) error {
				chain.Backend.Commit()
				return nil
			},
		},
		TxDecoding: seth.TxDecodingHooks{
			Pre: func(client *seth.Client) error {
				chain.Backend.Commit()
				return nil
			},
		},
	}

	sethClient, err := seth.NewClientBuilder().
		WithNetworkName("simulated").
		WithEthClient(chain.Backend.Client()).
		WithPrivateKeys([]string{TestPrivateKey}).
		WithProtections(false, false, seth.MustMakeDuration(1*time.Second)).
		WithHooks(hooks).
		Build()
	require.NoError(t, err, "failed to build simulated client")

	err = client.LoadContracts(logger, sethClient)
	require.NoError(t, err, "failed to load contracts")

	return sethClient
}
