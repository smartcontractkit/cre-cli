package settings_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
	"github.com/smartcontractkit/dev-platform/internal/testutil"
)

func createTestContext(t *testing.T, envVars map[string]string, targetDir string) (*viper.Viper, *zerolog.Logger) {
	envFilePath := filepath.Join(targetDir, constants.DefaultEnvFileName)
	require.NoError(t, godotenv.Write(envVars, envFilePath))

	v := viper.New()
	v.SetConfigFile(envFilePath)
	require.NoError(t, v.ReadInConfig())

	v.Set(settings.Flags.CliEnvFile.Name, envFilePath)
	logger := testutil.NewTestLogger()

	return v, logger
}

func setUpTestSettingsFiles(t *testing.T, v *viper.Viper, workflowTemplatePath string, TempProjectSettingsFile string, targetDir string) {
	// Workflow settings
	workflowFilePath := filepath.Join(targetDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath))
	v.Set(settings.Flags.CliSettingsFile.Name, workflowFilePath)

	// Project settings
	projectFilePath := filepath.Join(targetDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(TempProjectSettingsFile, projectFilePath))
	v.Set("projectSettingsPath", projectFilePath)

	t.Cleanup(func() {
		os.Remove(workflowFilePath)
		os.Remove(projectFilePath)
	})
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, srcFile)
	return err
}

func TestLoadEnvAndSettingsEmptyTarget(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar: "",
		settings.EthUrlEnvVar:    "http://localhost:8545",
	}
	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err)

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err)

	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreWorkingDirectory()

	v, logger := createTestContext(t, envVars, tempDir)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)
	s, err := settings.New(logger, v)

	assert.Error(t, err, "Expected error due to empty target")
	assert.Contains(t, err.Error(), "target not set", "Expected missing target error")
	assert.Nil(t, s, "Settings object should be nil on error")
}

func TestLoadEnvAndSettings(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "production-testnet",
		settings.EthUrlEnvVar:        "http://localhost:8545",
		settings.EthPrivateKeyEnvVar: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	}

	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err)

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err)

	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreWorkingDirectory()

	v, logger := createTestContext(t, envVars, tempDir)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)
	s, err := settings.New(logger, v)
	require.NoError(t, err)
	assert.Equal(t, "production-testnet", s.User.TargetName)
	assert.Equal(t, "http://localhost:8545", s.User.EthUrl)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestLoadEnvAndSettingsWithWorkflowSettingsFlag(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "production-testnet",
		settings.EthUrlEnvVar:        "http://localhost:8545",
		settings.EthPrivateKeyEnvVar: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	}

	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err)

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err)

	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreWorkingDirectory()

	v, logger := createTestContext(t, envVars, tempDir)

	tempWorkflowDir := t.TempDir()

	workflowFilePath := filepath.Join(tempWorkflowDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath))
	v.Set(settings.Flags.CliSettingsFile.Name, workflowFilePath)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)
	s, err := settings.New(logger, v)
	require.NoError(t, err)
	assert.Equal(t, "production-testnet", s.User.TargetName)
	assert.Equal(t, "http://localhost:8545", s.User.EthUrl)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestInlineEnvTakesPrecedenceOverDotEnv(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "Production",
		settings.EthUrlEnvVar:        "http://localhost:8545",
		settings.EthPrivateKeyEnvVar: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	}

	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err)

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err)

	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreWorkingDirectory()

	v, logger := createTestContext(t, envVars, tempDir)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)
	os.Setenv(settings.CreTargetEnvVar, "production-testnet")
	defer os.Unsetenv(settings.CreTargetEnvVar)
	s, err := settings.New(logger, v)
	require.NoError(t, err)
	assert.Equal(t, "production-testnet", s.User.TargetName)
	assert.Equal(t, "http://localhost:8545", s.User.EthUrl)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestLoadEnvAndMergedSettings(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "production-testnet",
		settings.EthUrlEnvVar:        "",
		settings.EthPrivateKeyEnvVar: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	}

	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err)

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err)

	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err)
	defer restoreWorkingDirectory()

	v, logger := createTestContext(t, envVars, tempDir)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)

	s, err := settings.New(logger, v)
	require.NoError(t, err)
	require.NotNil(t, s)

	assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, "Workflow owner address should be 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	assert.Equal(t, "workflowTest", s.Workflow.UserWorkflowSettings.WorkflowName, "Workflow name should be workflowTest")

	assert.Equal(t, 1, s.Workflow.DevPlatformSettings.DonID, "DonID should be 1")

	assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, "Workflow owner address should be taken from workflow settings")
	assert.Equal(t, "workflowTest", s.Workflow.UserWorkflowSettings.WorkflowName, "Workflow name should be taken from workflow settings")

	assert.Equal(t, "seth.toml", s.Workflow.LoggingSettings.SethConfigPath, "Logging seth config path should be set to 'seth.toml'")

	require.Len(t, s.Workflow.RPCs, 2, "There should be 2 RPC endpoints")
	rpc1 := s.Workflow.RPCs[0]
	rpc2 := s.Workflow.RPCs[1]
	assert.Equal(t, "https://somethingElse.rpc.org", rpc1.Url, "First RPC URL mismatch")
	assert.Equal(t, "https://something.rpc.org", rpc2.Url, "Second RPC URL mismatch")
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestWorkflowStorageSettings(t *testing.T) {
	tests := []struct {
		name                    string
		envVars                 map[string]string
		creSettingsFile         string
		workflowSettingsFile    string
		expectedStorageSettings settings.WorkflowStorageSettings
		expectError             bool
		wantErr                 string
	}{
		{
			name: "Empty storage settings",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:         "testdata/workflow_storage/project-no-storage.yaml",
			workflowSettingsFile:    "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{},
		},
		{
			name: "Storage settings only in project.yaml",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:      "testdata/workflow_storage/project-with-storage.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{
				Gist: settings.GistStorageSettings{
					GithubToken: "githubToken",
				},
			},
		},
		{
			name: "Storage settings only in workflow.yaml",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:      "testdata/workflow_storage/project-no-storage.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-with-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{
				Gist: settings.GistStorageSettings{
					GithubToken: "workflowGitHubToken",
				},
			},
		},
		{
			name: "Storage settings in both project.yaml and workflow.yaml",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:      "testdata/workflow_storage/project-with-storage.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-with-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{
				Gist: settings.GistStorageSettings{
					GithubToken: "workflowGitHubToken",
				},
			},
		},
		{
			name: "Hardcoded value for storage settings",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:     "production-testnet",
				settings.EthUrlEnvVar:        "http://localhost:8545",
			},
			creSettingsFile:      "testdata/workflow_storage/project-hardcoded-gh-token.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{
				Gist: settings.GistStorageSettings{
					GithubToken: "hardcodedToken",
				},
			},
		},
		{
			name: "Missing environment variable",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar: "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:     "production-testnet",
				settings.EthUrlEnvVar:        "http://localhost:8545",
				// Intentionally NOT setting settings.GithubApiTokenEnvVar
			},
			creSettingsFile:      "testdata/workflow_storage/project-with-storage.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-with-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{
				Gist: settings.GistStorageSettings{
					GithubToken: "",
				},
			},
		},
		{
			name: "Invalid storage YAML",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:         "testdata/workflow_storage/project-invalid-storage.yaml",
			workflowSettingsFile:    "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{},
		},
		{
			name: "Empty storage YAML",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:         "testdata/workflow_storage/project-empty-storage.yaml",
			workflowSettingsFile:    "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{},
		},
		{
			name: "Partial storage YAML",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:         "testdata/workflow_storage/project-partial-storage.yaml",
			workflowSettingsFile:    "testdata/workflow_storage/workflow-no-storage.yaml",
			expectedStorageSettings: settings.WorkflowStorageSettings{},
		},
		{
			name: "Malformed storage YAML",
			envVars: map[string]string{
				settings.EthPrivateKeyEnvVar:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				settings.CreTargetEnvVar:        "production-testnet",
				settings.EthUrlEnvVar:           "http://localhost:8545",
				settings.GithubApiTokenEnvVar:   "githubToken",
				"CRE_GITHUB_API_TOKEN_WORKFLOW": "workflowGitHubToken",
			},
			creSettingsFile:      "testdata/workflow_storage/project-malformed-storage.yaml",
			workflowSettingsFile: "testdata/workflow_storage/workflow-no-storage.yaml",
			expectError:          true,
			wantErr:              "failed to load project settings",
		},
	}

	oldToken := os.Getenv(settings.GithubApiTokenEnvVar)
	oldWorkflowToken := os.Getenv("CRE_GITHUB_API_TOKEN_WORKFLOW")
	t.Cleanup(func() {
		os.Setenv(settings.GithubApiTokenEnvVar, oldToken)
		os.Setenv("CRE_GITHUB_API_TOKEN_WORKFLOW", oldWorkflowToken)
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Clear environment variables for tests
			os.Unsetenv(settings.GithubApiTokenEnvVar)
			os.Unsetenv("CRE_GITHUB_API_TOKEN_WORKFLOW")

			envFilePath := filepath.Join(tempDir, ".env")
			err := godotenv.Write(tc.envVars, envFilePath)
			require.NoError(t, err, "Failed to write .env file")

			creSettingsPath := filepath.Join(tempDir, "project.yaml")
			err = copyFile(tc.creSettingsFile, creSettingsPath)
			require.NoError(t, err, "Failed to copy cre settings file")

			workflowSettingsPath := filepath.Join(tempDir, "workflow.yaml")
			err = copyFile(tc.workflowSettingsFile, workflowSettingsPath)
			require.NoError(t, err, "Failed to copy workflow settings file")

			v := viper.New()
			logger := testutil.NewTestLogger()

			// TODO: Settings code shouldn't depend on flag settings. Env file path should be passes as a parameter.
			v.Set(settings.Flags.CliEnvFile.Name, envFilePath)
			v.Set(settings.Flags.CliSettingsFile.Name, workflowSettingsPath)

			restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
			require.NoError(t, err)
			defer restoreWorkingDirectory()

			s, err := settings.New(logger, v)
			if tc.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Contains(t, err.Error(), tc.wantErr, "Error message should match expected")
			} else {
				require.NoError(t, err, "Failed to load settings")
				assert.Equal(t, tc.expectedStorageSettings, s.StorageSettings, "Storage settings should match expected")
			}
		})
	}
}
