package deploy

import (
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
)

func (h *handler) buildPrivateRegistryInput() privateregistryclient.WorkflowInRegistryInput {
	input := privateregistryclient.WorkflowInRegistryInput{
		WorkflowID:   h.workflowArtifact.WorkflowID,
		Status:       privateregistryclient.WorkflowStatusActive,
		WorkflowName: h.inputs.WorkflowName,
		BinaryURL:    h.inputs.BinaryURL,
		DonFamily:    h.inputs.DonFamily,
	}

	if configURL := h.inputs.ResolveConfigURL(""); configURL != "" {
		input.ConfigURL = &configURL
	}

	if h.inputs.WorkflowTag != "" {
		tag := h.inputs.WorkflowTag
		input.Tag = &tag
	}

	return input
}
