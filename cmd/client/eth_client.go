package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/keystone/generated/capabilities_registry"
	workflow_registry_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	cmdCommon "github.com/smartcontractkit/dev-platform/cmd/common"
	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
)

func LoadContracts(l *zerolog.Logger, client *seth.Client) error {

	abi, err := workflow_registry_wrapper.WorkflowRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get WorkflowRegistry ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.WorkflowRegistryContractName, *abi)
	client.ContractStore.AddBIN(constants.WorkflowRegistryContractName, common.FromHex(workflow_registry_wrapper.WorkflowRegistryMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.WorkflowRegistryContractName)

	abi, err = capabilities_registry.CapabilitiesRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get CapabilitiesRegistry ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.CapabilitiesRegistryContractName, *abi)
	client.ContractStore.AddBIN(constants.CapabilitiesRegistryContractName, common.FromHex(capabilities_registry.CapabilitiesRegistryMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.CapabilitiesRegistryContractName)

	return nil
}

func NewEthClientFromEnv(v *viper.Viper, l *zerolog.Logger, ethUrl string) (*seth.Client, error) {
	l.Debug().Msg("Setting up environment for executing on-chain transactions")

	// check configuration file then use default value
	sethConfigPath := v.GetString(settings.SethConfigPathSettingName)

	ethChainID, err := getChainID(ethUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	rawPrivKey := v.GetString(settings.EthPrivateKeyEnvVar)
	normPrivKey := settings.NormalizeHexKey(rawPrivKey)
	if normPrivKey == "" {
		l.Debug().Msg("No private key provided, all commands that write to chain will work only in unsigned mode")
	} else {
		if err := cmdCommon.ValidatePrivateKey(normPrivKey); err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
	}
	client, err := NewSethClient(sethConfigPath, ethUrl, []string{normPrivKey}, ethChainID)
	l.Debug().Str("Seth config", sethConfigPath).Uint64("Chain ID", ethChainID).Msg("Setting up connectivity client based on RPC URL and private key info")
	if err != nil {
		return nil, fmt.Errorf("failed to create Seth client: %w", err)
	}
	l.Debug().Int64("ChainID", client.ChainID).Str("URL", client.URL).Msg("Connected to a RPC node")

	l.Debug().Msg("Loading contract interfaces")
	err = LoadContracts(l, client)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract interfaces: %w", err)
	}

	l.Debug().Msg("Contract interaction client environment is ready to use")

	return client, nil
}

func NewSethClient(
	configFile string,
	rpc string,
	privateKeys []string,
	chainId uint64,
) (*seth.Client, error) {
	return NewSethClientWithSimulated(configFile, rpc, privateKeys, chainId, nil)
}

func NewSethClientWithSimulated(
	configFile string,
	rpc string,
	privateKeys []string,
	chainId uint64,
	backend *simulated.Backend,
) (*seth.Client, error) {
	var sethClientBuilder *seth.ClientBuilder
	var err error
	// if a config file is provided, we will use it to create the client
	if configFile != "" {
		sethConfig, readErr := readSethConfigFromFile(configFile)
		if readErr != nil {
			return nil, readErr
		}

		sethClientBuilder = seth.NewClientBuilderWithConfig(sethConfig).
			UseNetworkWithChainId(chainId).
			WithRpcUrl(rpc)
	} else {
		// if full flexibility is not needed we create a client with reasonable defaults
		// if you need to further tweak them, please refer to https://github.com/smartcontractkit/chainlink-testing-framework/blob/main/seth/README.md
		sethClientBuilder = seth.NewClientBuilder().
			WithProtections(true, false, seth.MustMakeDuration(1*time.Minute)).
			// Fast priority will add a 20% buffer on top of what the node suggests
			// we will use last 20 block to estimate block congestion and further bump gas price suggested by the node
			// we retry 10 times if gas estimation RPC calls fail
			WithGasPriceEstimations(true, 20, seth.Priority_Fast, 10)
		if rpc != "" {
			sethClientBuilder.WithRpcUrl(rpc)
		} else {
			sethClientBuilder.WithEthClient(backend.Client())
		}
	}

	// if private key is provided, we will use it to sign transactions
	// otherwise we will run in read-only mode
	if len(privateKeys) > 0 {
		sethClientBuilder.WithPrivateKeys(privateKeys)
	} else {
		sethClientBuilder.WithReadOnlyMode()
	}

	sethClient, err := sethClientBuilder.Build()

	return sethClient, err
}

func readSethConfigFromFile(configPath string) (*seth.Config, error) {
	d, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var sethConfig seth.Config
	err = toml.Unmarshal(d, &sethConfig)
	if err != nil {
		return nil, err
	}

	return &sethConfig, nil
}

func getChainID(rpcURL string) (uint64, error) {
	client, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	var chainID string
	err = client.CallContext(context.Background(), &chainID, "eth_chainId")
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(chainID, 0, 64)
}
