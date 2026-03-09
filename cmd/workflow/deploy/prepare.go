package deploy

import (
	"encoding/base64"
	"fmt"
	"os"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"
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
	}
	h.log.Debug().Msg("Workflow config is ready")
	return configData, nil
}

func (h *handler) PrepareWorkflowArtifact() error {
	var binaryForID []byte

	if h.urlBinaryData != nil {
		// URL case: binary fetched from URL, used directly for WorkflowID.
		binaryForID = h.urlBinaryData
	} else {
		binaryData, err := h.prepareWorkflowBinary()
		if err != nil {
			return err
		}
		h.workflowArtifact.BinaryData = binaryData

		// The binary data read from file is base64 encoded, so we decode before generating the workflow ID.
		// Ref https://github.com/smartcontractkit/chainlink/blob/a4adc900d98d4e6eec0a6f80fcf86d883a8f1e3c/core/services/workflows/artifacts/v2/store.go#L211-L213
		binaryDataDecoded, err := base64.StdEncoding.DecodeString(string(binaryData))
		if err != nil {
			return fmt.Errorf("failed to decode base64 binary data: %w", err)
		}
		binaryForID = binaryDataDecoded
	}

	var configData []byte
	if h.urlConfigData != nil {
		configData = h.urlConfigData
	} else {
		var err error
		configData, err = h.prepareWorkflowConfig()
		if err != nil {
			return err
		}
		h.workflowArtifact.ConfigData = configData
	}

	workflowID, err := workflowUtils.GenerateWorkflowIDFromStrings(h.inputs.WorkflowOwner, h.inputs.WorkflowName, binaryForID, configData, "")
	if err != nil {
		return fmt.Errorf("failed to generate workflow ID: %w", err)
	}

	h.workflowArtifact.WorkflowID = workflowID

	return nil
}
