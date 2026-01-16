package deploy

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

// DeploymentResult represents the result of deploying a single contract
type DeploymentResult struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
	TxHash  string `yaml:"tx_hash"`
}

// ContractDeployer handles the deployment of contracts
type ContractDeployer struct {
	log           *zerolog.Logger
	client        *seth.Client
	contractsPath string
}

// NewContractDeployer creates a new contract deployer
func NewContractDeployer(log *zerolog.Logger, client *seth.Client, contractsPath string) *ContractDeployer {
	return &ContractDeployer{
		log:           log,
		client:        client,
		contractsPath: contractsPath,
	}
}

// Deploy deploys a single contract using its Go bindings from the ContractStore
func (d *ContractDeployer) Deploy(config ContractConfig) (*DeploymentResult, error) {
	d.log.Debug().
		Str("contract", config.Name).
		Str("package", config.Package).
		Int("constructor_args", len(config.Constructor)).
		Msg("Starting contract deployment")

	// Check if contract is in the ContractStore
	contractABI, ok := d.client.ContractStore.GetABI(config.Name)
	if !ok {
		return nil, fmt.Errorf("contract %s not found in ContractStore. Make sure to load the contract bindings first", config.Name)
	}

	bytecode, hasBytecode := d.client.ContractStore.GetBIN(config.Name)
	if !hasBytecode || len(bytecode) == 0 {
		return nil, fmt.Errorf("contract %s has no bytecode in ContractStore", config.Name)
	}

	// Parse constructor arguments
	args, err := d.parseConstructorArgs(config, contractABI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse constructor arguments: %w", err)
	}

	// Deploy the contract
	txOpts := d.client.NewTXOpts()

	var deployedContract seth.DeploymentData
	if len(args) == 0 {
		// No constructor arguments
		deployedContract, err = d.client.DeployContractFromContractStore(txOpts, config.Name)
	} else {
		// With constructor arguments
		deployedContract, err = d.client.DeployContractFromContractStore(txOpts, config.Name, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	// Get the transaction hash from the deployment
	txHash := ""
	if deployedContract.Transaction != nil {
		txHash = deployedContract.Transaction.Hash().Hex()
	}

	return &DeploymentResult{
		Name:    config.Name,
		Address: deployedContract.Address.Hex(),
		TxHash:  txHash,
	}, nil
}

// parseConstructorArgs converts constructor arguments from config to Go types
func (d *ContractDeployer) parseConstructorArgs(config ContractConfig, contractABI *abi.ABI) ([]interface{}, error) {
	if len(contractABI.Constructor.Inputs) == 0 {
		if len(config.Constructor) > 0 {
			return nil, fmt.Errorf("contract has no constructor arguments but %d were provided", len(config.Constructor))
		}
		return nil, nil
	}

	if len(config.Constructor) != len(contractABI.Constructor.Inputs) {
		return nil, fmt.Errorf("expected %d constructor arguments, got %d",
			len(contractABI.Constructor.Inputs), len(config.Constructor))
	}

	args := make([]interface{}, len(config.Constructor))
	for i, arg := range config.Constructor {
		abiArg := contractABI.Constructor.Inputs[i]

		// Validate type matches (loosely)
		if !typesMatch(arg.Type, abiArg.Type.String()) {
			d.log.Warn().
				Str("config_type", arg.Type).
				Str("abi_type", abiArg.Type.String()).
				Int("arg_index", i).
				Msg("Type mismatch warning - proceeding with ABI type")
		}

		parsedArg, err := parseArgValue(arg.Value, abiArg.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument %d (%s): %w", i, abiArg.Name, err)
		}
		args[i] = parsedArg
	}

	return args, nil
}

// typesMatch checks if the config type matches the ABI type
func typesMatch(configType, abiType string) bool {
	// Normalize types for comparison
	configType = strings.ToLower(strings.TrimSpace(configType))
	abiType = strings.ToLower(strings.TrimSpace(abiType))

	// Handle common aliases
	if configType == "uint" {
		configType = "uint256"
	}
	if configType == "int" {
		configType = "int256"
	}

	return configType == abiType
}

// parseArgValue parses a string value into the appropriate Go type based on ABI type
func parseArgValue(value string, abiType abi.Type) (interface{}, error) {
	switch abiType.T {
	case abi.AddressTy:
		if !common.IsHexAddress(value) {
			return nil, fmt.Errorf("invalid address: %s", value)
		}
		return common.HexToAddress(value), nil

	case abi.UintTy, abi.IntTy:
		n := new(big.Int)
		n, ok := n.SetString(value, 0)
		if !ok {
			return nil, fmt.Errorf("invalid integer: %s", value)
		}

		// Convert to appropriate size based on ABI type
		switch abiType.Size {
		case 8:
			if abiType.T == abi.UintTy {
				if !n.IsUint64() || n.Uint64() > 255 {
					return nil, fmt.Errorf("value %s overflows uint8", value)
				}
				return uint8(n.Uint64()), nil //nolint:gosec // bounds checked above
			}
			if !n.IsInt64() || n.Int64() < -128 || n.Int64() > 127 {
				return nil, fmt.Errorf("value %s overflows int8", value)
			}
			return int8(n.Int64()), nil //nolint:gosec // bounds checked above
		case 16:
			if abiType.T == abi.UintTy {
				if !n.IsUint64() || n.Uint64() > 65535 {
					return nil, fmt.Errorf("value %s overflows uint16", value)
				}
				return uint16(n.Uint64()), nil //nolint:gosec // bounds checked above
			}
			if !n.IsInt64() || n.Int64() < -32768 || n.Int64() > 32767 {
				return nil, fmt.Errorf("value %s overflows int16", value)
			}
			return int16(n.Int64()), nil //nolint:gosec // bounds checked above
		case 32:
			if abiType.T == abi.UintTy {
				if !n.IsUint64() || n.Uint64() > 4294967295 {
					return nil, fmt.Errorf("value %s overflows uint32", value)
				}
				return uint32(n.Uint64()), nil //nolint:gosec // bounds checked above
			}
			if !n.IsInt64() || n.Int64() < -2147483648 || n.Int64() > 2147483647 {
				return nil, fmt.Errorf("value %s overflows int32", value)
			}
			return int32(n.Int64()), nil //nolint:gosec // bounds checked above
		case 64:
			if abiType.T == abi.UintTy {
				return n.Uint64(), nil
			}
			return n.Int64(), nil
		default:
			// For larger integers, return *big.Int
			return n, nil
		}

	case abi.BoolTy:
		switch strings.ToLower(value) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean: %s", value)
		}

	case abi.StringTy:
		return value, nil

	case abi.BytesTy:
		return common.FromHex(value), nil

	case abi.FixedBytesTy:
		bytes := common.FromHex(value)
		if len(bytes) != abiType.Size {
			return nil, fmt.Errorf("bytes%d requires exactly %d bytes, got %d", abiType.Size, abiType.Size, len(bytes))
		}
		// Convert to fixed-size array
		return convertToFixedBytes(bytes, abiType.Size)

	case abi.SliceTy:
		// For array types, we'd need to parse JSON arrays
		return nil, fmt.Errorf("array types not yet supported in constructor arguments")

	default:
		return nil, fmt.Errorf("unsupported type: %s", abiType.String())
	}
}

// convertToFixedBytes converts a byte slice to a fixed-size byte array
func convertToFixedBytes(bytes []byte, size int) (interface{}, error) {
	switch size {
	case 1:
		var arr [1]byte
		copy(arr[:], bytes)
		return arr, nil
	case 2:
		var arr [2]byte
		copy(arr[:], bytes)
		return arr, nil
	case 4:
		var arr [4]byte
		copy(arr[:], bytes)
		return arr, nil
	case 8:
		var arr [8]byte
		copy(arr[:], bytes)
		return arr, nil
	case 16:
		var arr [16]byte
		copy(arr[:], bytes)
		return arr, nil
	case 32:
		var arr [32]byte
		copy(arr[:], bytes)
		return arr, nil
	default:
		return nil, fmt.Errorf("unsupported fixed bytes size: %d", size)
	}
}
