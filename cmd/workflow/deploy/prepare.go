package deploy

import "github.com/smartcontractkit/cre-cli/internal/artifact"

func (h *handler) PrepareWorkflowArtifact() (err error) {
	h.workflowArtifact, err = h.artifactBuilder.Build(artifact.Inputs{
		WorkflowOwner: h.inputs.WorkflowOwner,
		WorkflowName:  h.inputs.WorkflowName,
		OutputPath:    h.inputs.OutputPath,
		ConfigPath:    h.inputs.ConfigPath,
	})
	if err != nil {
		return err
	}

	h.log.Info().Str("workflowID", h.workflowArtifact.WorkflowID).Msg("Prepared workflow artifact")

	return nil
}
