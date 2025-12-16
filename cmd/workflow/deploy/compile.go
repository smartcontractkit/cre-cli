package deploy

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/build"
)

func (h *handler) Compile() error {
	buildParams, err := build.ResolveBuildParamsForWorkflow(h.inputs.WorkflowPath, h.inputs.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve build inputs: %w", err)
	}
	h.runtimeContext.Workflow.Language = buildParams.WorkflowLanguage

	if err := h.builder.CompileAndSave(buildParams); err != nil {
		return fmt.Errorf("failed to compile workflow: %w", err)
	}

	return nil
}
