package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

func TestFindProjectSettingsPath(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (string, func())
		expectFound   bool
		expectError   bool
		errorContains string
	}{
		{
			name: "finds project.yaml in current directory",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				projectFile := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
				err := os.WriteFile(projectFile, []byte("test: content"), 0600)
				require.NoError(t, err)
				return tempDir, func() {}
			},
			expectFound: true,
			expectError: false,
		},
		{
			name: "finds project.yaml in parent directory",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				subDir := filepath.Join(tempDir, "subdir")
				err := os.MkdirAll(subDir, 0755)
				require.NoError(t, err)

				projectFile := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
				err = os.WriteFile(projectFile, []byte("test: content"), 0600)
				require.NoError(t, err)
				return subDir, func() {}
			},
			expectFound: true,
			expectError: false,
		},
		{
			name: "no project.yaml found",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				return tempDir, func() {}
			},
			expectFound: false,
			expectError: false,
		},
		{
			name: "empty start directory",
			setupFunc: func() (string, func()) {
				return "", func() {}
			},
			expectFound:   false,
			expectError:   true,
			errorContains: "starting directory cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDir, cleanup := tt.setupFunc()
			defer cleanup()

			path, found, err := FindProjectSettingsPath(startDir)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectFound, found)

			if tt.expectFound {
				assert.NotEmpty(t, path)
				assert.Contains(t, path, constants.DefaultProjectSettingsFileName)
				_, err := os.Stat(path)
				assert.NoError(t, err, "project settings file should exist at returned path")
			} else {
				assert.Empty(t, path)
			}
		})
	}
}

func TestSetProjectContext(t *testing.T) {
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		// Restore original working directory
		_ = os.Chdir(originalWd)
	}()

	tests := []struct {
		name          string
		setupFunc     func() (string, func())
		projectPath   string
		expectError   bool
		errorContains string
	}{
		{
			name: "sets project context with explicit path",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				return tempDir, func() {}
			},
			projectPath: "", // This will be set to tempDir below in the test execution
			expectError: false,
		},
		{
			name: "sets project context finding project.yaml",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				subDir := filepath.Join(tempDir, "subdir")
				err := os.MkdirAll(subDir, 0755)
				require.NoError(t, err)

				projectFile := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
				err = os.WriteFile(projectFile, []byte("test: content"), 0600)
				require.NoError(t, err)

				// Change to subdirectory so we test finding parent
				_ = os.Chdir(subDir)
				return tempDir, func() {}
			},
			projectPath: "", // Empty path should trigger search
			expectError: false,
		},
		{
			name: "fails when no project.yaml found",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				_ = os.Chdir(tempDir)
				return "", func() {}
			},
			projectPath:   "", // Empty path should trigger search
			expectError:   true,
			errorContains: "no project settings file found",
		},
		{
			name: "fails when project path doesn't exist",
			setupFunc: func() (string, func()) {
				return "", func() {}
			},
			projectPath:   "/nonexistent/path",
			expectError:   true,
			errorContains: "failed to change directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedDir, cleanup := tt.setupFunc()
			defer cleanup()

			// For the explicit path test, use the expectedDir as projectPath
			projectPath := tt.projectPath
			if tt.name == "sets project context with explicit path" {
				projectPath = expectedDir
			}

			err := SetProjectContext(projectPath)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
				return
			}

			require.NoError(t, err)

			if expectedDir != "" {
				currentDir, err := os.Getwd()
				require.NoError(t, err)
				// Resolve symlinks for both paths before comparison
				resolvedCurrentDir, err := filepath.EvalSymlinks(currentDir)
				require.NoError(t, err)
				resolvedExpectedDir, err := filepath.EvalSymlinks(expectedDir)
				require.NoError(t, err)
				assert.Equal(t, resolvedExpectedDir, resolvedCurrentDir)
			}
		})
	}
}

func TestSetExecutionContext(t *testing.T) {
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		// Restore original working directory
		_ = os.Chdir(originalWd)
	}()

	logger := zerolog.Nop()

	tests := []struct {
		name            string
		setupFunc       func() (string, func())
		cmdSetup        func() *cobra.Command
		args            []string
		projectRootFlag string
		expectError     bool
		errorContains   string
	}{
		{
			name: "sets context for non-workflow command",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				projectFile := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
				err := os.WriteFile(projectFile, []byte("test: content"), 0600)
				require.NoError(t, err)
				return tempDir, func() {}
			},
			cmdSetup: func() *cobra.Command {
				return &cobra.Command{Use: "test"}
			},
			args:            []string{},
			projectRootFlag: "",
			expectError:     false,
		},
		{
			name: "sets context with explicit project root flag",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				return tempDir, func() {}
			},
			cmdSetup: func() *cobra.Command {
				return &cobra.Command{Use: "test"}
			},
			args:        []string{},
			expectError: false,
		},
		{
			name: "fails when project root flag path doesn't exist",
			setupFunc: func() (string, func()) {
				return "", func() {}
			},
			cmdSetup: func() *cobra.Command {
				return &cobra.Command{Use: "test"}
			},
			args:            []string{},
			projectRootFlag: "/nonexistent/path",
			expectError:     true,
			errorContains:   "project root path does not exist",
		},
		{
			name: "sets context for workflow command with workflow arg",
			setupFunc: func() (string, func()) {
				tempDir := t.TempDir()
				projectFile := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
				err := os.WriteFile(projectFile, []byte("test: content"), 0600)
				require.NoError(t, err)

				// Create a workflow directory with workflow.yaml
				workflowDir := filepath.Join(tempDir, "test-workflow")
				err = os.MkdirAll(workflowDir, 0755)
				require.NoError(t, err)

				workflowFile := filepath.Join(workflowDir, constants.DefaultWorkflowSettingsFileName)
				workflowContent := `
test-profile:
  user-workflow:
    workflow-name: test-workflow
`
				err = os.WriteFile(workflowFile, []byte(workflowContent), 0600)
				require.NoError(t, err)

				return tempDir, func() {}
			},
			cmdSetup: func() *cobra.Command {
				workflowCmd := &cobra.Command{Use: "workflow"}
				subCmd := &cobra.Command{Use: "test"}
				workflowCmd.AddCommand(subCmd)
				return subCmd
			},
			args:        []string{"test-workflow"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedDir, cleanup := tt.setupFunc()
			defer cleanup()

			// Set the projectRootFlag to expectedDir if not explicitly set and expectedDir exists
			projectRootFlag := tt.projectRootFlag
			if projectRootFlag == "" && expectedDir != "" {
				projectRootFlag = expectedDir
			}

			cmd := tt.cmdSetup()
			projectRoot, err := SetExecutionContext(cmd, tt.args, projectRootFlag, &logger)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.ErrorContains(t, err, tt.errorContains)
				}
				return
			}

			require.NoError(t, err)

			// Verify project root was returned correctly
			if expectedDir != "" {
				resolvedProjectRoot, err := filepath.EvalSymlinks(projectRoot)
				require.NoError(t, err)
				resolvedExpectedDir, err := filepath.EvalSymlinks(expectedDir)
				require.NoError(t, err)
				assert.Equal(t, resolvedExpectedDir, resolvedProjectRoot, "returned project root should match expected directory")
			}

			// Verify we're in the correct directory
			if expectedDir != "" {
				currentDir, err := os.Getwd()
				require.NoError(t, err)

				// For workflow commands with args, we should be in a subdirectory
				if IsWorkflowCommand(cmd) && len(tt.args) == 1 {
					// Resolve symlinks for both paths before comparison
					resolvedCurrentDir, err := filepath.EvalSymlinks(currentDir)
					require.NoError(t, err)
					resolvedExpectedDir, err := filepath.EvalSymlinks(expectedDir)
					require.NoError(t, err)
					assert.Contains(t, resolvedCurrentDir, resolvedExpectedDir)
				} else {
					// Resolve symlinks for both paths before comparison
					resolvedCurrentDir, err := filepath.EvalSymlinks(currentDir)
					require.NoError(t, err)
					resolvedExpectedDir, err := filepath.EvalSymlinks(expectedDir)
					require.NoError(t, err)
					assert.Equal(t, resolvedExpectedDir, resolvedCurrentDir)
				}
			}
		})
	}
}
