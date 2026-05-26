package settings

import (
	"testing"

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
