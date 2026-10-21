package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/keystone/generated/balance_reader"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/latest/werc20_mock"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/mock_forwarder"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/reserve_manager"
	workflow_registry_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	crpc "github.com/smartcontractkit/cre-cli/internal/rpc"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func LoadContracts(l *zerolog.Logger, client *seth.Client) error {

	abi, err := workflow_registry_wrapper.WorkflowRegistryMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get WorkflowRegistry ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.WorkflowRegistryContractName, *abi)
	client.ContractStore.AddBIN(constants.WorkflowRegistryContractName, common.FromHex(workflow_registry_wrapper.WorkflowRegistryMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.WorkflowRegistryContractName)

	abi, err = balance_reader.BalanceReaderMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get BalanceReader ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.BalanceReaderContractName, *abi)
	client.ContractStore.AddBIN(constants.BalanceReaderContractName, common.FromHex(balance_reader.BalanceReaderMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.BalanceReaderContractName)

	abi, err = werc20_mock.WERC20MockMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get WERC20Mock ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.WERC20MockContractName, *abi)
	client.ContractStore.AddBIN(constants.WERC20MockContractName, common.FromHex(werc20_mock.WERC20MockMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.WERC20MockContractName)

	abi, err = reserve_manager.ReserveManagerMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get ReserveManager ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.ReserveManagerContractName, *abi)
	client.ContractStore.AddBIN(constants.ReserveManagerContractName, common.FromHex(reserve_manager.ReserveManagerMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.ReserveManagerContractName)

	abi, err = mock_forwarder.MockKeystoneForwarderMetaData.GetAbi()
	if err != nil {
		return fmt.Errorf("failed to get MockKeystoneForwarder ABI: %w", err)
	}
	client.ContractStore.AddABI(constants.MockKeystoneForwarderContractName, *abi)
	client.ContractStore.AddBIN(constants.MockKeystoneForwarderContractName, common.FromHex(mock_forwarder.MockKeystoneForwarderMetaData.Bin))
	l.Debug().Msgf("Loaded %s contract into ContractStore", constants.MockKeystoneForwarderContractName)

	return nil
}

func NewEthClientFromEnv(v *viper.Viper, l *zerolog.Logger, ethUrl string) (*seth.Client, error) {
	l.Debug().Msg("Setting up environment for executing on-chain transactions")

	// check configuration file then use default value
	sethConfigPath := v.GetString(settings.SethConfigPathSettingName)

	cleartextOpts := crpc.CleartextPolicyOptions{
		AllowInsecure: v.GetBool(settings.Flags.AllowInsecureRPC.Name),
	}
	if warnMsg, blockErr := crpc.EvaluateCleartextRPC(ethUrl, cleartextOpts); blockErr != nil {
		return nil, blockErr
	} else if warnMsg != "" {
		l.Warn().Str("url", crpc.RedactURL(ethUrl)).Msg(warnMsg)
	}

	ethChainID, err := getChainID(ethUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	resolvedKey, err := settings.ResolveEthPrivateKeyFromEnv(v.GetString(settings.EthPrivateKeyEnvVar))
	if err != nil {
		return nil, err
	}

	keys := []string{}
	if !resolvedKey.IsSet() {
		l.Debug().Msg("No private key provided, all commands that write to chain will work only in unsigned mode")
	} else {
		keys = []string{resolvedKey.Hex()}
	}

	client, err := NewSethClient(sethConfigPath, ethUrl, keys, ethChainID)
	l.Debug().Str("Seth config", sethConfigPath).Uint64("Chain ID", ethChainID).Msg("Setting up connectivity client based on RPC URL and private key info")
	if err != nil {
		return nil, fmt.Errorf("failed to create Seth client: %w", err)
	}
	l.Debug().Int64("ChainID", client.ChainID).Str("URL", crpc.RedactURL(client.URL)).Msg("Connected to a RPC node")

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
	rpcURL string,
	privateKeys []string,
	chainId uint64,
	backend *simulated.Backend,
) (*seth.Client, error) {
	var sethClientBuilder *seth.ClientBuilder
	var rpcHeaders http.Header
	// if a config file is provided, we will use it to create the client
	if configFile != "" {
		sethConfig, readErr := readSethConfigFromFile(configFile)
		if readErr != nil {
			return nil, readErr
		}

		sethClientBuilder = seth.NewClientBuilderWithConfig(sethConfig).
			UseNetworkWithChainId(chainId)
		rpcHeaders = sethConfig.RPCHeaders
		// The RPC connection is passed in through WithEthClient below, and seth rejects configs that set both.
		if sethConfig.Network != nil {
			sethConfig.Network.URLs = nil
		}
	} else {
		// if full flexibility is not needed we create a client with reasonable defaults
		// if you need to further tweak them, please refer to https://github.com/smartcontractkit/chainlink-testing-framework/blob/main/seth/README.md
		sethClientBuilder = seth.NewClientBuilder().
			WithProtections(false, false, seth.MustMakeDuration(1*time.Minute)).
			// Auto priority will use what the blockchain node suggests
			// estimationBlocks and attemptCount can be disregarded for Auto priority
			WithGasPriceEstimations(false, 20, seth.Priority_Auto, 1)
	}

	// Never let seth dial the RPC URL itself: its transport sets InsecureSkipVerify, so TLS certificates
	// would not be checked. We dial with go-ethereum's default transport instead, which verifies them.
	var rpcClient *rpc.Client
	switch {
	case rpcURL != "":
		var dialErr error
		rpcClient, dialErr = dialRPC(rpcURL, rpcHeaders)
		if dialErr != nil {
			return nil, dialErr
		}
		sethClientBuilder.WithEthClient(ethclient.NewClient(rpcClient))
	case backend != nil:
		sethClientBuilder.WithEthClient(backend.Client())
	default:
		return nil, errors.New("no RPC URL provided")
	}

	// if private key is provided, we will use it to sign transactions
	// otherwise we will run in read-only mode
	if len(privateKeys) > 0 {
		sethClientBuilder.WithPrivateKeys(privateKeys)
	} else {
		sethClientBuilder.WithReadOnlyMode()
	}

	sethClient, err := sethClientBuilder.Build()
	if err != nil {
		if rpcClient != nil {
			rpcClient.Close()
		}
		return nil, err
	}

	if rpcURL != "" {
		if err := restoreRPCURL(sethClient, rpcURL); err != nil {
			rpcClient.Close()
			return nil, err
		}
	}

	return sethClient, nil
}

// dialRPC connects to the RPC node using go-ethereum's default HTTP transport, which verifies TLS certificates.
func dialRPC(rpcURL string, headers http.Header) (*rpc.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rpcClient, err := rpc.DialOptions(ctx, rpcURL, rpc.WithHeaders(headers))
	if err != nil {
		return nil, fmt.Errorf("failed to connect RPC client to '%s': %w", crpc.RedactURL(rpcURL), err)
	}
	return rpcClient, nil
}

// restoreRPCURL puts back what seth only sets up when it dials the URL itself: the client URL and the tracer
// used to decode reverted transactions. The tracer opens its own connection with go-ethereum's default
// transport, so certificates are verified there too.
func restoreRPCURL(sethClient *seth.Client, rpcURL string) error {
	sethClient.URL = rpcURL
	sethClient.Cfg.Network.URLs = []string{rpcURL}

	if sethClient.Cfg.TracingLevel == seth.TracingLevel_None || sethClient.Tracer != nil {
		return nil
	}
	tracer, err := seth.NewTracer(sethClient.ContractStore, sethClient.ABIFinder, sethClient.Cfg, sethClient.ContractAddressToNameMap, sethClient.Addresses)
	if err != nil {
		return fmt.Errorf("failed to create tracer: %w", err)
	}
	sethClient.Tracer = tracer
	return nil
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

// TODO(DEVSVCS-5178)
func getChainID(rpcURL string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	var chainID string
	err = client.CallContext(ctx, &chainID, "eth_chainId")
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(chainID, 0, 64)
}
