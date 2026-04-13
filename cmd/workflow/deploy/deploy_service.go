package deploy

import (
	"errors"
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// runDeploy orchestrates the deploy flow common to all registry targets:
// pre-deploy checks → artifact upload → registry upsert.
func runDeploy(adapter registryAdapter, h *handler) error {
	if err := adapter.RunPreDeployChecks(); err != nil {
		if errors.Is(err, errDeployHalted) {
			return nil
		}
		return err
	}

	ui.Line()
	ui.Dim("Uploading files...")
	if err := h.uploadArtifacts(); err != nil {
		return fmt.Errorf("failed to upload workflow: %w", err)
	}

	return adapter.Upsert()
}
