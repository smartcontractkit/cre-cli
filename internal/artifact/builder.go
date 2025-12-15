package artifact

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"
)

type Inputs struct {
	WorkflowOwner string
	WorkflowName  string
	OutputPath    string
	ConfigPath    string
}

type Builder struct {
	log *zerolog.Logger
}

func NewBuilder(log *zerolog.Logger) *Builder {
	return &Builder{
		log: log,
	}
}

func (b *Builder) Build(inputs Inputs) (art *Artifact, err error) {
	art = &Artifact{}

	if inputs.WorkflowOwner == "" {
		return nil, fmt.Errorf("workflow owner is required")
	}

	if inputs.WorkflowName == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	art.ConfigData, err = b.prepareWorkflowConfig(inputs.ConfigPath)
	if err != nil {
		return nil, err
	}

	art.BinaryData, err = b.prepareWorkflowBinary(inputs.OutputPath)
	if err != nil {
		return nil, err
	}

	binaryDataDecoded, err := decodeBinaryData(art.BinaryData)
	if err != nil {
		return nil, err
	}

	art.WorkflowID, err = workflowUtils.GenerateWorkflowIDFromStrings(inputs.WorkflowOwner, inputs.WorkflowName, binaryDataDecoded, art.ConfigData, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow ID: %w", err)
	}

	return art, nil
}

func decodeBinaryData(binaryData []byte) ([]byte, error) {
	// Note: the binary data read from file is base64 encoded, so we need to decode it before generating the workflow ID.
	// This matches the behavior in the Chainlink node. Ref https://github.com/smartcontractkit/chainlink/blob/a4adc900d98d4e6eec0a6f80fcf86d883a8f1e3c/core/services/workflows/artifacts/v2/store.go#L211-L213
	binaryDataDecoded := make([]byte, base64.StdEncoding.DecodedLen(len(binaryData)))
	if _, err := base64.StdEncoding.Decode(binaryDataDecoded, binaryData); err != nil {
		return nil, fmt.Errorf("failed to decode base64 binary data: %w", err)
	}
	return binaryDataDecoded, nil
}

func (b *Builder) prepareWorkflowBinary(outputPath string) ([]byte, error) {
	b.log.Debug().Str("Binary Path", outputPath).Msg("Fetching workflow binary")
	binaryData, err := os.ReadFile(outputPath)
	if err != nil {
		b.log.Error().Err(err).Str("path", outputPath).Msg("Failed to read output file")
		return nil, err
	}
	b.log.Debug().Msg("Workflow binary WASM is ready")
	return binaryData, nil
}

func (b *Builder) prepareWorkflowConfig(configPath string) ([]byte, error) {
	b.log.Debug().Str("Config Path", configPath).Msg("Fetching workflow config")
	var configData []byte
	var err error
	if configPath != "" {
		configData, err = os.ReadFile(configPath)
		if err != nil {
			b.log.Error().Err(err).Str("path", configPath).Msg("Failed to read config file")
			return nil, err
		}
	}
	b.log.Debug().Msg("Workflow config is ready")
	return configData, nil
}
