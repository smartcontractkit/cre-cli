package testutil

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

//go:embed testdata/test-workflow.yaml
var testWorkflowSettingsContent string

//go:embed testdata/test-project.yaml
var testProjectSettingsContent string

func NewTestSettings(v *viper.Viper, logger *zerolog.Logger) (*settings.Settings, error) {
	tmpDir, err := os.MkdirTemp("", "test-settings")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	workflowFile := filepath.Join(tmpDir, "test-workflow.yaml")
	if err := os.WriteFile(workflowFile, []byte(testWorkflowSettingsContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write workflow settings file: %w", err)
	}
	projectFile := filepath.Join(tmpDir, "test-project.yaml")
	if err := os.WriteFile(projectFile, []byte(testProjectSettingsContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write project settings file: %w", err)
	}

	v.Set(settings.Flags.CliSettingsFile.Name, workflowFile)
	v.Set("projectSettingsPath", projectFile)
	v.Set(settings.CreTargetEnvVar, "production-testnet")

	cmd := &cobra.Command{Use: "login"}
	testSettings, err := settings.New(logger, v, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create new test settings: %w", err)
	}

	return testSettings, nil
}
