package settings_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
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
	v.Set(settings.Flags.ProjectRoot.Name, workflowFilePath)

	// Project settings
	projectFilePath := filepath.Join(targetDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(TempProjectSettingsFile, projectFilePath))

	// Create dummy artifact files that are referenced in the workflow settings
	mainGoPath := filepath.Join(targetDir, "main.go")
	require.NoError(t, os.WriteFile(mainGoPath, []byte("package main\n"), 0600))

	configPath := filepath.Join(targetDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0600))

	t.Cleanup(func() {
		os.Remove(workflowFilePath)
		os.Remove(projectFilePath)
		os.Remove(mainGoPath)
		os.Remove(configPath)
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
	cmd := &cobra.Command{Use: "login"}
	s, err := settings.New(logger, v, cmd, "")

	assert.Error(t, err, "Expected error due to empty target")
	assert.Contains(t, err.Error(), "target not set", "Expected missing target error")
	assert.Nil(t, s, "Settings object should be nil on error")
}

func TestLoadEnvAndSettings(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "staging",
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
	cmd := &cobra.Command{Use: "login"}
	s, err := settings.New(logger, v, cmd, "")
	require.NoError(t, err)
	assert.Equal(t, "staging", s.User.TargetName)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestLoadEnvAndSettingsWithWorkflowSettingsFlag(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "staging",
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
	v.Set(settings.Flags.ProjectRoot.Name, workflowFilePath)

	setUpTestSettingsFiles(t, v, workflowTemplatePath, projectTemplatePath, tempDir)
	cmd := &cobra.Command{Use: "login"}
	s, err := settings.New(logger, v, cmd, "")
	require.NoError(t, err)
	assert.Equal(t, "staging", s.User.TargetName)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestInlineEnvTakesPrecedenceOverDotEnv(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "Production",
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
	os.Setenv(settings.CreTargetEnvVar, "staging")
	defer os.Unsetenv(settings.CreTargetEnvVar)

	cmd := &cobra.Command{Use: "login"}
	s, err := settings.New(logger, v, cmd, "")
	require.NoError(t, err)
	assert.Equal(t, "staging", s.User.TargetName)
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", s.User.EthPrivateKey)
}

func TestLoadEnvAndMergedSettings(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "staging",
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

	cmd := &cobra.Command{Use: "workflow"}
	s, err := settings.New(logger, v, cmd, "")
	require.NoError(t, err)
	require.NotNil(t, s)

	assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, "Workflow owner address should be 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	assert.Equal(t, "workflowTest", s.Workflow.UserWorkflowSettings.WorkflowName, "Workflow name should be workflowTest")

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

// helper to build a command with optional --broadcast flag and parse args
func makeCmd(use string, defineBroadcast bool, args ...string) *cobra.Command {
	cmd := &cobra.Command{
		Use: use,
		Run: func(cmd *cobra.Command, args []string) {},
	}
	if defineBroadcast {
		cmd.Flags().Bool("broadcast", false, "broadcast the tx")
	}
	_ = cmd.Flags().Parse(args) // parse only the provided flag args
	return cmd
}

func TestLoadEnvAndSettingsInvalidTarget(t *testing.T) {
	envVars := map[string]string{
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

	v.Set(settings.Flags.Target.Name, "nonexistent-target")

	cmd := &cobra.Command{Use: "workflow"}
	s, err := settings.New(logger, v, cmd, "")

	assert.Error(t, err, "Expected error due to invalid target")
	assert.Contains(t, err.Error(), "target not found: nonexistent-target", "Expected target not found error")
	assert.Nil(t, s, "Settings object should be nil on error")
}

func TestShouldSkipGetOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      *cobra.Command
		wantSkip bool
	}{
		{
			name:     "simulate with --broadcast=true → do NOT skip",
			cmd:      makeCmd("simulate", true, "--broadcast"),
			wantSkip: false,
		},
		{
			name:     "simulate with --broadcast=false → skip",
			cmd:      makeCmd("simulate", true, "--broadcast=false"),
			wantSkip: true,
		},
		{
			name:     "simulate with broadcast flag defined but not set → skip",
			cmd:      makeCmd("simulate", true /* no args */),
			wantSkip: true,
		},
		{
			name:     "simulate with no broadcast flag defined → skip (treated as false)",
			cmd:      makeCmd("simulate", false /* no flag defined */),
			wantSkip: true,
		},
		{
			name:     "non-simulate command with broadcast=true → do NOT skip",
			cmd:      makeCmd("deploy", true, "--broadcast"),
			wantSkip: false,
		},
		{
			name:     "non-simulate command with no broadcast → do NOT skip",
			cmd:      makeCmd("deploy", false),
			wantSkip: false,
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := settings.ShouldSkipGetOwner(tc.cmd)
			if got != tc.wantSkip {
				t.Fatalf("ShouldSkipGetOwner(%q) = %v, want %v", tc.cmd.Name(), got, tc.wantSkip)
			}
		})
	}
}

func TestArtifactPathValidation(t *testing.T) {
	envVars := map[string]string{
		settings.CreTargetEnvVar:     "staging",
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

	// Set up workflow and project settings files, but DON'T create the artifact files
	workflowFilePath := filepath.Join(tempDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath))
	v.Set(settings.Flags.ProjectRoot.Name, workflowFilePath)

	projectFilePath := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(projectTemplatePath, projectFilePath))

	cmd := &cobra.Command{Use: "workflow"}
	s, err := settings.New(logger, v, cmd, "")

	// Assert that we get an error about the missing workflow path
	assert.Error(t, err, "Expected error due to missing artifact path")
	assert.Contains(t, err.Error(), "WorkflowPath does not exist", "Expected error message about missing WorkflowPath")
	assert.Contains(t, err.Error(), "./main.go", "Expected error message to include the file path")
	assert.Contains(t, err.Error(), "staging", "Expected error message to include the target name")
	assert.Nil(t, s, "Settings object should be nil on error")
}
