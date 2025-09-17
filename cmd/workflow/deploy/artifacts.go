package deploy

import (
	"fmt"

	"github.com/smartcontractkit/dev-platform/internal/client/graphqlclient"
	"github.com/smartcontractkit/dev-platform/internal/client/storageclient"
)

func (h *handler) UploadArtifacts() error {
	if h.workflowArtifact == nil {
		return fmt.Errorf("workflowArtifact is nil")
	}
	binaryData := h.workflowArtifact.BinaryData
	configData := h.workflowArtifact.ConfigData
	workflowID := h.workflowArtifact.WorkflowID

	configURL := storageclient.GenerateUnsignedGetUrlForArtifactResponse{
		UnsignedGetUrl: "",
	}

	gql := graphqlclient.New(h.credentials, h.environmentSet)
	storageClient := storageclient.New(gql, h.environmentSet.WorkflowRegistryAddress, h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, h.environmentSet.WorkflowRegistryChainSelector, h.log)
	if h.settings.StorageSettings.CREStorage.ServiceTimeout != 0 {
		storageClient.SetServiceTimeout(h.settings.StorageSettings.CREStorage.ServiceTimeout)
	}
	if h.settings.StorageSettings.CREStorage.HTTPTimeout != 0 {
		storageClient.SetHTTPTimeout(h.settings.StorageSettings.CREStorage.HTTPTimeout)
	}

	binaryURL, err := storageClient.UploadArtifactWithRetriesAndGetURL(
		workflowID, storageclient.ArtifactTypeBinary, binaryData, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("uploading binary artifact: %w", err)
	}
	h.log.Debug().Str("URL", binaryURL.UnsignedGetUrl).Msg("Successfully uploaded workflow binary to CRE Storage Service")
	if len(configData) > 0 {
		configURL, err = storageClient.UploadArtifactWithRetriesAndGetURL(
			workflowID, storageclient.ArtifactTypeConfig, configData, "text/plain")
		if err != nil {
			return fmt.Errorf("uploading config artifact: %w", err)
		}
		h.log.Debug().Str("URL", configURL.UnsignedGetUrl).Msg("Successfully uploaded workflow config to CRE Storage Service")
	}
	h.log.Info().Msg("Successfully uploaded workflow artifacts to CRE Storage Service")

	h.inputs.BinaryURL = binaryURL.UnsignedGetUrl
	h.inputs.ConfigURL = &configURL.UnsignedGetUrl
	return nil
}
