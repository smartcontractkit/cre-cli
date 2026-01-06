package deploy

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// ContractsConfig represents the structure of contracts.yaml
type ContractsConfig struct {
	Chain     string           `yaml:"chain"`
	Contracts []ContractConfig `yaml:"contracts"`
}

// ContractConfig represents a single contract configuration
type ContractConfig struct {
	Name        string           `yaml:"name"`
	Package     string           `yaml:"package"`
	Deploy      bool             `yaml:"deploy"`
	Constructor []ConstructorArg `yaml:"constructor"`
}

// ConstructorArg represents a constructor argument
type ConstructorArg struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

// ParseContractsConfig reads and parses the contracts.yaml file
func ParseContractsConfig(path string) (*ContractsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ContractsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// Validate validates the contracts configuration
func (c *ContractsConfig) Validate() error {
	if strings.TrimSpace(c.Chain) == "" {
		return fmt.Errorf("chain is required")
	}

	// Validate chain name is valid
	if err := settings.IsValidChainName(c.Chain); err != nil {
		return fmt.Errorf("invalid chain name: %w", err)
	}

	if len(c.Contracts) == 0 {
		return fmt.Errorf("at least one contract must be defined")
	}

	seenNames := make(map[string]bool)
	for i, contract := range c.Contracts {
		if strings.TrimSpace(contract.Name) == "" {
			return fmt.Errorf("contract[%d]: name is required", i)
		}

		if seenNames[contract.Name] {
			return fmt.Errorf("duplicate contract name: %s", contract.Name)
		}
		seenNames[contract.Name] = true

		if strings.TrimSpace(contract.Package) == "" {
			return fmt.Errorf("contract[%d] (%s): package is required", i, contract.Name)
		}

		// Validate constructor arguments
		for j, arg := range contract.Constructor {
			if strings.TrimSpace(arg.Type) == "" {
				return fmt.Errorf("contract[%d] (%s): constructor[%d]: type is required", i, contract.Name, j)
			}

			if !isValidSolidityType(arg.Type) {
				return fmt.Errorf("contract[%d] (%s): constructor[%d]: invalid type %q", i, contract.Name, j, arg.Type)
			}
		}
	}

	return nil
}

// GetContractsToDeploy returns contracts that have deploy: true
func (c *ContractsConfig) GetContractsToDeploy() []ContractConfig {
	var contracts []ContractConfig
	for _, contract := range c.Contracts {
		if contract.Deploy {
			contracts = append(contracts, contract)
		}
	}
	return contracts
}

// GetContractByName returns a contract by name
func (c *ContractsConfig) GetContractByName(name string) *ContractConfig {
	for _, contract := range c.Contracts {
		if contract.Name == name {
			return &contract
		}
	}
	return nil
}

// isValidSolidityType checks if the type is a valid Solidity type
func isValidSolidityType(t string) bool {
	validTypes := map[string]bool{
		// Basic types
		"address": true,
		"bool":    true,
		"string":  true,
		"bytes":   true,

		// Integers
		"int":    true,
		"int8":   true,
		"int16":  true,
		"int32":  true,
		"int64":  true,
		"int128": true,
		"int256": true,

		"uint":    true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"uint128": true,
		"uint256": true,

		// Fixed-size bytes
		"bytes1":  true,
		"bytes2":  true,
		"bytes4":  true,
		"bytes8":  true,
		"bytes16": true,
		"bytes32": true,
	}

	// Check for array types (e.g., "address[]", "uint256[]")
	if strings.HasSuffix(t, "[]") {
		baseType := strings.TrimSuffix(t, "[]")
		return validTypes[baseType]
	}

	return validTypes[t]
}
