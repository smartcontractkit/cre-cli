package deploy

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// DeployedContracts represents the structure of deployed_contracts.yaml
type DeployedContracts struct {
	ChainID   uint64                      `yaml:"chain_id"`
	ChainName string                      `yaml:"chain_name"`
	Timestamp string                      `yaml:"timestamp"`
	Contracts map[string]DeployedContract `yaml:"contracts"`
}

// DeployedContract represents a deployed contract entry
type DeployedContract struct {
	Address string `yaml:"address"`
	TxHash  string `yaml:"tx_hash"`
}

// WriteDeployedContracts writes the deployment results to deployed_contracts.yaml
func WriteDeployedContracts(path string, chainName string, results []DeploymentResult) error {
	// Get chain ID from chain name
	chainID, err := settings.GetChainSelectorByChainName(chainName)
	if err != nil {
		return fmt.Errorf("failed to get chain ID for %s: %w", chainName, err)
	}

	deployed := DeployedContracts{
		ChainID:   chainID,
		ChainName: chainName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Contracts: make(map[string]DeployedContract),
	}

	for _, result := range results {
		deployed.Contracts[result.Name] = DeployedContract{
			Address: result.Address,
			TxHash:  result.TxHash,
		}
	}

	data, err := yaml.Marshal(deployed)
	if err != nil {
		return fmt.Errorf("failed to marshal deployed contracts: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write deployed contracts file: %w", err)
	}

	return nil
}

// ReadDeployedContracts reads the deployed_contracts.yaml file
func ReadDeployedContracts(path string) (*DeployedContracts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployed contracts file: %w", err)
	}

	var deployed DeployedContracts
	if err := yaml.Unmarshal(data, &deployed); err != nil {
		return nil, fmt.Errorf("failed to parse deployed contracts: %w", err)
	}

	return &deployed, nil
}

// GetContractAddress returns the address of a deployed contract by name
func (d *DeployedContracts) GetContractAddress(name string) (string, error) {
	contract, ok := d.Contracts[name]
	if !ok {
		return "", fmt.Errorf("contract %s not found in deployed contracts", name)
	}
	return contract.Address, nil
}
