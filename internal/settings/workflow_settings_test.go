package settings

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowPathFromRaw(t *testing.T) {
	t.Parallel()

	t.Run("workflow-path under staging-settings", func(t *testing.T) {
		t.Parallel()
		raw := map[string]interface{}{
			"staging-settings": map[string]interface{}{
				"workflow-artifacts": map[string]interface{}{
					"workflow-path": "main.go",
				},
			},
		}
		path, err := workflowPathFromRaw(raw)
		require.NoError(t, err)
		assert.Equal(t, "main.go", path)
	})

	t.Run("workflow-path under custom target production-jovay", func(t *testing.T) {
		t.Parallel()
		raw := map[string]interface{}{
			"production-jovay": map[string]interface{}{
				"workflow-artifacts": map[string]interface{}{
					"workflow-path": "main.go",
				},
			},
		}
		path, err := workflowPathFromRaw(raw)
		require.NoError(t, err)
		assert.Equal(t, "main.go", path)
	})

	t.Run("no workflow-path anywhere", func(t *testing.T) {
		t.Parallel()
		raw := map[string]interface{}{
			"staging-settings": map[string]interface{}{
				"workflow-artifacts": map[string]interface{}{
					"config-path": "/tmp",
				},
			},
		}
		_, err := workflowPathFromRaw(raw)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow-path not found")
	})

	t.Run("multiple targets with workflow-path returns one", func(t *testing.T) {
		t.Parallel()
		raw := map[string]interface{}{
			"staging-settings": map[string]interface{}{
				"workflow-artifacts": map[string]interface{}{
					"workflow-path": "staging.go",
				},
			},
			"production-settings": map[string]interface{}{
				"workflow-artifacts": map[string]interface{}{
					"workflow-path": "production.go",
				},
			},
		}
		path, err := workflowPathFromRaw(raw)
		require.NoError(t, err)
		assert.True(t, path == "staging.go" || path == "production.go", "got %q", path)
	})
}

func TestLoadWorkflowSettingsValidatesWorkflowName(t *testing.T) {
	t.Parallel()

	const target = "staging"

	// A command named "hash" makes ShouldSkipGetOwner return true, so the loader skips owner
	// derivation and we can exercise workflow-name validation in isolation.
	newViper := func(t *testing.T, setName bool, name string) *viper.Viper {
		t.Helper()
		v := viper.New()
		v.Set(Flags.Target.Name, target)
		v.Set(target+".user-workflow.workflow-owner-address", "0x0000000000000000000000000000000000000001")
		if setName {
			v.Set(target+"."+WorkflowNameSettingName, name)
		}
		return v
	}

	tests := []struct {
		name      string
		setName   bool
		value     string
		wantError bool
	}{
		{name: "valid name loads", setName: true, value: "my-workflow", wantError: false},
		{name: "absent name still loads", setName: false, wantError: false},
		{name: "empty name still loads", setName: true, value: "", wantError: false},
		{name: "quote injects yaml", setName: true, value: "a\"\ninjected: true", wantError: true},
		{name: "newline", setName: true, value: "a\ninjected: true", wantError: true},
		{name: "path traversal", setName: true, value: "../../escape", wantError: true},
		{name: "space", setName: true, value: "my workflow", wantError: true},
		{name: "command substitution", setName: true, value: "a$(id)", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zerolog.Nop()
			cmd := &cobra.Command{Use: "hash"}
			v := newViper(t, tt.setName, tt.value)

			got, err := loadWorkflowSettings(&logger, v, cmd, "")

			if tt.wantError {
				require.Error(t, err, "expected workflow-name %q to be rejected", tt.value)
				assert.Contains(t, err.Error(), WorkflowNameSettingName,
					"error should name the offending setting")
				assert.Contains(t, err.Error(), target, "error should name the target")
			} else {
				require.NoError(t, err, "expected workflow-name %q to be accepted", tt.value)
				assert.Equal(t, tt.value, got.UserWorkflowSettings.WorkflowName)
			}
		})
	}
}
