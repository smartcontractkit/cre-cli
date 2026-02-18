package deploy

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	"github.com/smartcontractkit/cre-cli/internal/placeholder"
)

type workflowArtifact struct {
	BinaryData []byte
	ConfigData []byte
	WorkflowID string
}

func (h *handler) prepareWorkflowBinary() ([]byte, error) {
	h.log.Debug().Str("Binary Path", h.inputs.OutputPath).Msg("Fetching workflow binary")
	binaryData, err := os.ReadFile(h.inputs.OutputPath)
	if err != nil {
		h.log.Error().Err(err).Str("path", h.inputs.OutputPath).Msg("Failed to read output file")
		return nil, err
	}
	h.log.Debug().Msg("Workflow binary WASM is ready")
	return binaryData, nil
}

func (h *handler) prepareWorkflowConfig() ([]byte, error) {
	h.log.Debug().Str("Config Path", h.inputs.ConfigPath).Msg("Fetching workflow config")
	var configData []byte
	var err error
	if h.inputs.ConfigPath != "" {
		configData, err = os.ReadFile(h.inputs.ConfigPath)
		if err != nil {
			h.log.Error().Err(err).Str("path", h.inputs.ConfigPath).Msg("Failed to read config file")
			return nil, err
		}

		// Apply placeholder substitution for deployed contract addresses
		configData, err = h.substituteContractPlaceholders(configData)
		if err != nil {
			return nil, fmt.Errorf("failed to substitute contract placeholders: %w", err)
		}
	}
	h.log.Debug().Msg("Workflow config is ready")
	return configData, nil
}

// substituteContractPlaceholders replaces {{contracts.Name.address}} placeholders with deployed addresses
func (h *handler) substituteContractPlaceholders(configData []byte) ([]byte, error) {
	// Find project root by looking for parent directory containing project.yaml
	workflowDir := filepath.Dir(h.inputs.WorkflowPath)
	projectRoot := findProjectRoot(workflowDir)
	if projectRoot == "" {
		h.log.Debug().Msg("Project root not found, skipping placeholder substitution")
		return configData, nil
	}

	substitutor, err := placeholder.NewSubstitutor(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create placeholder substitutor: %w", err)
	}

	if !substitutor.HasDeployedContracts() {
		// Check if there are placeholders that need substitution
		placeholders := placeholder.FindPlaceholders(string(configData))
		if len(placeholders) > 0 {
			return nil, fmt.Errorf("found %d contract placeholder(s) in config but no deployed_contracts.yaml found. Run 'cre contract deploy' first", len(placeholders))
		}
		h.log.Debug().Msg("No deployed_contracts.yaml found, skipping placeholder substitution")
		return configData, nil
	}

	substituted, err := substitutor.SubstituteString(string(configData))
	if err != nil {
		return nil, err
	}

	// Log substitutions that were made
	contracts := substitutor.GetAllDeployedContracts()
	if len(contracts) > 0 {
		h.log.Debug().Int("contracts", len(contracts)).Msg("Substituted contract address placeholders")
		for name, addr := range contracts {
			h.log.Debug().Str("contract", name).Str("address", addr).Msg("Placeholder substitution")
		}
	}

	return []byte(substituted), nil
}

// findProjectRoot searches upward from the given directory for a project.yaml file
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		projectFile := filepath.Join(dir, "project.yaml")
		if _, err := os.Stat(projectFile); err == nil {
			return dir
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			// Reached filesystem root
			return ""
		}
		dir = parentDir
	}
}

func (h *handler) PrepareWorkflowArtifact() error {
	var err error
	binaryData, err := h.prepareWorkflowBinary()
	if err != nil {
		return err
	}

	configData, err := h.prepareWorkflowConfig()
	if err != nil {
		return err
	}

	// Note: the binary data read from file is base64 encoded, so we need to decode it before generating the workflow ID.
	// This matches the behavior in the Chainlink node. Ref https://github.com/smartcontractkit/chainlink/blob/a4adc900d98d4e6eec0a6f80fcf86d883a8f1e3c/core/services/workflows/artifacts/v2/store.go#L211-L213
	binaryDataDecoded, err := base64.StdEncoding.DecodeString(string(binaryData))
	if err != nil {
		return fmt.Errorf("failed to decode base64 binary data: %w", err)
	}

	workflowID, err := workflowUtils.GenerateWorkflowIDFromStrings(h.inputs.WorkflowOwner, h.inputs.WorkflowName, binaryDataDecoded, configData, "")
	if err != nil {
		return fmt.Errorf("failed to generate workflow ID: %w", err)
	}

	h.workflowArtifact.BinaryData = binaryData
	h.workflowArtifact.ConfigData = configData
	h.workflowArtifact.WorkflowID = workflowID

	return nil
}
