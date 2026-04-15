package deploy

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/storageclient"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func (h *handler) uploadArtifacts() error {
	if h.workflowArtifact == nil {
		return fmt.Errorf("workflowArtifact is nil")
	}
	if h.inputs.WorkflowOwner == "" {
		return fmt.Errorf("workflow owner is empty")
	}

	// User-provided URLs (via --wasm URL / --config URL) skip the corresponding uploads.
	binaryFromURL := h.urlBinaryData != nil && h.inputs.BinaryURL != ""
	configFromURL := h.urlConfigData != nil && h.inputs.ConfigURL != nil && *h.inputs.ConfigURL != ""

	// When both artifacts come from user-provided URLs, no uploads needed at all.
	if binaryFromURL && (configFromURL || len(h.workflowArtifact.ConfigData) == 0) {
		return nil
	}

	binaryData := h.workflowArtifact.BinaryData
	configData := h.workflowArtifact.ConfigData
	workflowID := h.workflowArtifact.WorkflowID

	configURL := storageclient.GenerateUnsignedGetUrlForArtifactResponse{
		UnsignedGetUrl: "",
	}

	gql := graphqlclient.New(h.credentials, h.environmentSet, h.log)

	chainSelector, err := settings.GetChainSelectorByChainName(h.environmentSet.WorkflowRegistryChainName)
	if err != nil {
		return fmt.Errorf("failed to get chain selector for chain %q: %w", h.environmentSet.WorkflowRegistryChainName, err)
	}

	storageClient := storageclient.New(gql, h.environmentSet.WorkflowRegistryAddress, h.inputs.WorkflowOwner, chainSelector, h.log)
	if h.settings.StorageSettings.CREStorage.ServiceTimeout != 0 {
		storageClient.SetServiceTimeout(h.settings.StorageSettings.CREStorage.ServiceTimeout)
	}
	if h.settings.StorageSettings.CREStorage.HTTPTimeout != 0 {
		storageClient.SetHTTPTimeout(h.settings.StorageSettings.CREStorage.HTTPTimeout)
	}

	if !binaryFromURL {
		ui.Success(fmt.Sprintf("Loaded binary from: %s", h.inputs.OutputPath))
		binaryResp, err := storageClient.UploadArtifactWithRetriesAndGetURL(
			workflowID, storageclient.ArtifactTypeBinary, binaryData, "application/octet-stream")
		if err != nil {
			return fmt.Errorf("uploading binary artifact: %w", err)
		}
		ui.Success(fmt.Sprintf("Uploaded binary to: %s", binaryResp.UnsignedGetUrl))
		h.log.Debug().Str("URL", binaryResp.UnsignedGetUrl).Msg("Successfully uploaded workflow binary to CRE Storage Service")
		h.inputs.BinaryURL = binaryResp.UnsignedGetUrl
	}

	if !configFromURL && len(configData) > 0 {
		ui.Success(fmt.Sprintf("Loaded config from: %s", h.inputs.ConfigPath))
		configURL, err = storageClient.UploadArtifactWithRetriesAndGetURL(
			workflowID, storageclient.ArtifactTypeConfig, configData, "text/plain")
		if err != nil {
			return fmt.Errorf("uploading config artifact: %w", err)
		}
		ui.Success(fmt.Sprintf("Uploaded config to: %s", configURL.UnsignedGetUrl))
		h.log.Debug().Str("URL", configURL.UnsignedGetUrl).Msg("Successfully uploaded workflow config to CRE Storage Service")
	}

	if !configFromURL {
		h.inputs.ConfigURL = &configURL.UnsignedGetUrl
	}
	return nil
}
