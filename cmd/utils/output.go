package utils

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"

	workflow_registry_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
)

const (
	RawOutputFormat  = "raw"
	JsonOutputFormat = "json"
	YamlOutputFormat = "yaml"
)

type WorkflowMetadataSerialized struct {
	WorkflowName  string `yaml:"name" json:"name"`
	WorkflowID    string `yaml:"workflowID" json:"workflowID"`
	WorkflowOwner string `yaml:"workflowOwner" json:"workflowOwner"`
	BinaryURL     string `yaml:"binaryURL" json:"binaryURL"`
	ConfigURL     string `yaml:"configURL" json:"configURL"`
	DonFamily     string `yaml:"donFamily" json:"donFamily"`
	Status        string `yaml:"status" json:"status"`
}

func getStatusString(status uint8) string {
	if status == 1 {
		return "Paused"
	}
	return "Active"
}

func SerializeToStruct(metadata []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView) []WorkflowMetadataSerialized {
	var serializedMetadata []WorkflowMetadataSerialized
	for _, m := range metadata {
		serializedMetadata = append(serializedMetadata, WorkflowMetadataSerialized{
			WorkflowName:  m.WorkflowName,
			WorkflowID:    hex.EncodeToString(m.WorkflowId[:]),
			WorkflowOwner: m.Owner.Hex(),
			BinaryURL:     m.BinaryUrl,
			ConfigURL:     m.ConfigUrl,
			DonFamily:     m.DonFamily,
			Status:        getStatusString(m.Status),
		})
	}
	return serializedMetadata
}

func HandleJsonOrYamlFormat(
	log *zerolog.Logger,
	format string,
	workflowMetadata []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView,
	outputPath string,
) error {
	structMetadata := SerializeToStruct(workflowMetadata)

	var out []byte
	var err error
	outputFileSerialized := true

	switch format {
	case JsonOutputFormat:
		out, err = json.MarshalIndent(structMetadata, "", "  ")
	case YamlOutputFormat:
		out, err = yaml.Marshal(structMetadata)
	default:
		return errors.New("format not supported")
	}

	if err != nil {
		outputFileSerialized = false
		msg := fmt.Sprintf("Could not serialize workflow metadata as %s. "+
			"Please try again or use a different format.", strings.ToUpper(format))
		log.Info().Msg(msg)
	}

	if outputPath == "" {
		fmt.Printf("\n# Workflow metadata in %s format:\n\n%s\n", strings.ToUpper(format), string(out))
		return nil
	}

	log.Info().
		Str("Output path", outputPath).
		Msgf("Preparing to write workflow metadata to an output %s file", strings.ToUpper(format))

	err = os.WriteFile(outputPath, out, 0600)
	if err != nil {
		outputFileSerialized = false
		msg := "Could not write workflow metadata to a file. " +
			"Check that the path exists and that you have write permissions."
		log.Info().Msg(msg)
	}

	if outputFileSerialized {
		log.Info().
			Str("Output path", outputPath).
			Msg("Workflow metadata written to the output file successfully")
	}

	return nil
}
