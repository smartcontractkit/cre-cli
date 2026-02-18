package placeholder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	contractsFolder       = "contracts"
	deployedContractsFile = "deployed_contracts.yaml"
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

// placeholderPattern matches {{contracts.ContractName.address}} or {{contracts.ContractName.tx_hash}}
var placeholderPattern = regexp.MustCompile(`\{\{contracts\.([a-zA-Z0-9_]+)\.(address|tx_hash)\}\}`)

// Substitutor handles placeholder substitution in configuration files
type Substitutor struct {
	projectRoot string
	deployed    *DeployedContracts
}

// NewSubstitutor creates a new placeholder substitutor
func NewSubstitutor(projectRoot string) (*Substitutor, error) {
	deployedPath := filepath.Join(projectRoot, contractsFolder, deployedContractsFile)

	// Check if deployed_contracts.yaml exists
	if _, err := os.Stat(deployedPath); os.IsNotExist(err) {
		// No deployed contracts file - return substitutor that does nothing
		return &Substitutor{
			projectRoot: projectRoot,
			deployed:    nil,
		}, nil
	}

	// Read deployed contracts
	data, err := os.ReadFile(deployedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployed contracts: %w", err)
	}

	var deployed DeployedContracts
	if err := yaml.Unmarshal(data, &deployed); err != nil {
		return nil, fmt.Errorf("failed to parse deployed contracts: %w", err)
	}

	return &Substitutor{
		projectRoot: projectRoot,
		deployed:    &deployed,
	}, nil
}

// HasDeployedContracts returns true if deployed_contracts.yaml was found
func (s *Substitutor) HasDeployedContracts() bool {
	return s.deployed != nil
}

// SubstituteString replaces placeholders in a string with deployed contract values
func (s *Substitutor) SubstituteString(content string) (string, error) {
	if s.deployed == nil {
		return content, nil
	}

	var substitutionErrors []string

	result := placeholderPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract contract name and field from the match
		submatches := placeholderPattern.FindStringSubmatch(match)
		if len(submatches) != 3 {
			substitutionErrors = append(substitutionErrors, fmt.Sprintf("invalid placeholder format: %s", match))
			return match
		}

		contractName := submatches[1]
		field := submatches[2]

		contract, ok := s.deployed.Contracts[contractName]
		if !ok {
			substitutionErrors = append(substitutionErrors, fmt.Sprintf("contract %q not found in deployed_contracts.yaml", contractName))
			return match
		}

		switch field {
		case "address":
			return contract.Address
		case "tx_hash":
			return contract.TxHash
		default:
			substitutionErrors = append(substitutionErrors, fmt.Sprintf("unknown field %q for contract %q", field, contractName))
			return match
		}
	})

	if len(substitutionErrors) > 0 {
		return "", fmt.Errorf("placeholder substitution errors: %s", strings.Join(substitutionErrors, "; "))
	}

	return result, nil
}

// SubstituteFile reads a file, substitutes placeholders, and returns the modified content
func (s *Substitutor) SubstituteFile(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	substituted, err := s.SubstituteString(string(content))
	if err != nil {
		return nil, err
	}

	return []byte(substituted), nil
}

// SubstituteFileInPlace reads a file, substitutes placeholders, and writes back
func (s *Substitutor) SubstituteFileInPlace(filePath string) error {
	content, err := s.SubstituteFile(filePath)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, content, 0600)
}

// FindPlaceholders returns all placeholders found in the content
func FindPlaceholders(content string) []string {
	matches := placeholderPattern.FindAllString(content, -1)
	return matches
}

// ValidatePlaceholders checks if all placeholders in the content can be resolved
func (s *Substitutor) ValidatePlaceholders(content string) error {
	if s.deployed == nil {
		placeholders := FindPlaceholders(content)
		if len(placeholders) > 0 {
			return fmt.Errorf("found %d placeholder(s) but no deployed_contracts.yaml exists. Run 'cre contract deploy' first", len(placeholders))
		}
		return nil
	}

	_, err := s.SubstituteString(content)
	return err
}

// GetDeployedContractAddress returns the address of a deployed contract
func (s *Substitutor) GetDeployedContractAddress(contractName string) (string, error) {
	if s.deployed == nil {
		return "", fmt.Errorf("no deployed contracts available")
	}

	contract, ok := s.deployed.Contracts[contractName]
	if !ok {
		return "", fmt.Errorf("contract %q not found in deployed contracts", contractName)
	}

	return contract.Address, nil
}

// GetAllDeployedContracts returns all deployed contract names and addresses
func (s *Substitutor) GetAllDeployedContracts() map[string]string {
	if s.deployed == nil {
		return nil
	}

	result := make(map[string]string)
	for name, contract := range s.deployed.Contracts {
		result[name] = contract.Address
	}
	return result
}
