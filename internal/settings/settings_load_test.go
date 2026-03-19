package settings_test

import (
	"bytes"
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

var TempWorkflowSettingsFile = filepath.Join("testdata", "workflow_storage", "workflow-with-hierarchy.yaml")
var TempProjectSettingsFile = filepath.Join("testdata", "workflow_storage", "project-with-hierarchy.yaml")

func createBlankCommand() *cobra.Command {
	return &cobra.Command{
		Use: "workflow",
	}
}

func newBufferLogger() (*zerolog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	return &logger, &buf
}

func TestSettingsHierarchy(t *testing.T) {
	// Get absolute paths for template files
	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving workflow template path")

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err, "Error when resolving project template path")

	// Create temporary directory and change to it
	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err, "Error changing working directory")
	defer restoreWorkingDirectory()

	// Copy test files to the temporary directory
	workflowFilePath := filepath.Join(tempDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath), "Error copying workflow file")

	projectFilePath := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(projectTemplatePath, projectFilePath), "Error copying project file")

	// Set up viper and load settings
	blankCmd := createBlankCommand()
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")

	err = settings.LoadSettingsIntoViper(v, blankCmd)
	require.NoError(t, err, "Error when loading settings")

	hierarchyVal := v.GetString("staging.hierarchy-test")
	require.Equal(t, "Workflow", hierarchyVal)

	testVal := v.GetString("staging.test-key")
	require.Equal(t, "workflowValue", testVal)
}

// TODO: happy path unit test, write more edge case tests
func TestLoadingSettingsForValidFile(t *testing.T) {
	// Get absolute paths for template files
	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving workflow template path")

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err, "Error when resolving project template path")

	// Create temporary directory and change to it
	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err, "Error changing working directory")
	defer restoreWorkingDirectory()

	// Copy test files to the temporary directory
	workflowFilePath := filepath.Join(tempDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath), "Error copying workflow file")

	projectFilePath := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(projectTemplatePath, projectFilePath), "Error copying project file")

	// Set up viper and load settings
	blankCmd := createBlankCommand()
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")

	err = settings.LoadSettingsIntoViper(v, blankCmd)
	require.NoError(t, err, "Error when loading settings")

	rpcUrl, err := settings.GetRpcUrlSettings(v, "ethereum-testnet-sepolia-arbitrum-1")
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://somethingElse.rpc.org", rpcUrl)

	rpcUrl, err = settings.GetRpcUrlSettings(v, "ethereum-testnet-sepolia")
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://something.rpc.org", rpcUrl)
}

func TestLoadEnvFromParent(t *testing.T) {
	tempDir := t.TempDir()

	parentDir := filepath.Join(tempDir, "envparent")
	err := os.Mkdir(parentDir, 0755)
	require.NoError(t, err, "unable to create parent directory")

	childDir, err := os.MkdirTemp(parentDir, "envchild")
	require.NoError(t, err, "unable to create temporary child directory")

	envFilePath := filepath.Join(parentDir, constants.DefaultEnvFileName)
	envContent := "TEST_VAR=from_parent\n"
	err = os.WriteFile(envFilePath, []byte(envContent), 0600)
	require.NoError(t, err, "unable to write .env file")

	t.Cleanup(func() {
		os.Unsetenv("TEST_VAR")
		os.Unsetenv("CRE_TARGET")
	})

	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(childDir)
	require.NoError(t, err, "unable to change working directory to child directory")
	defer restoreWorkingDirectory()

	absChildDir, err := filepath.Abs(childDir)
	require.NoError(t, err, "unable to resolve absolute path")

	found, err := settings.FindEnvFile(absChildDir, constants.DefaultEnvFileName)
	require.NoError(t, err, "FindEnvFile() failed to find the .env file from a parent directory")

	logger := testutil.NewTestLogger()
	v := viper.New()
	settings.LoadEnv(logger, v, found)

	require.Equal(t, "from_parent", os.Getenv("TEST_VAR"), "TEST_VAR should have been loaded from the .env file")
	require.Empty(t, os.Getenv("TARGET"), "TARGET should not be set in the configuration")
}

func TestLoadEnvEmptyPath(t *testing.T) {
	logger, buf := newBufferLogger()
	v := viper.New()

	settings.LoadEnv(logger, v, "")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "No environment file specified")
	assert.Contains(t, logOutput, "was not found")
	assert.Contains(t, logOutput, "MUST be exported")

	assert.Empty(t, settings.LoadedEnvFilePath(), "no file should be recorded when path is empty")
	assert.Nil(t, settings.LoadedEnvVars(), "no vars should be recorded when path is empty")
}

func TestLoadEnvInvalidFile(t *testing.T) {
	logger, buf := newBufferLogger()
	v := viper.New()

	settings.LoadEnv(logger, v, "/nonexistent/path/.env")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "Not able to load configuration from environment file")
	assert.Contains(t, logOutput, "MUST be exported")
	assert.Contains(t, logOutput, "dotenvx.com/docs/env-file")

	assert.Empty(t, settings.LoadedEnvFilePath(), "no file should be recorded when load fails")
	assert.Nil(t, settings.LoadedEnvVars(), "no vars should be recorded when load fails")
}

func TestLoadEnvSuccess(t *testing.T) {
	tempDir := t.TempDir()
	envFilePath := filepath.Join(tempDir, ".env")
	envVars := map[string]string{
		"CRE_TARGET":          "staging",
		"CRE_ETH_PRIVATE_KEY": "abc123",
		"GOTOOLCHAIN":         "go1.25.3",
	}
	require.NoError(t, godotenv.Write(envVars, envFilePath))

	t.Cleanup(func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	})

	logger, buf := newBufferLogger()
	v := viper.New()
	settings.LoadEnv(logger, v, envFilePath)

	// Verify env vars were set in the process environment
	assert.Equal(t, "staging", os.Getenv("CRE_TARGET"))
	assert.Equal(t, "abc123", os.Getenv("CRE_ETH_PRIVATE_KEY"))
	assert.Equal(t, "go1.25.3", os.Getenv("GOTOOLCHAIN"))

	// Verify Viper has the bound sensitive vars
	assert.Equal(t, "staging", v.GetString("CRE_TARGET"))
	assert.Equal(t, "abc123", v.GetString("CRE_ETH_PRIVATE_KEY"))

	// Verify state tracking
	assert.Equal(t, envFilePath, settings.LoadedEnvFilePath())
	require.NotNil(t, settings.LoadedEnvVars())
	assert.Equal(t, "staging", settings.LoadedEnvVars()["CRE_TARGET"])
	assert.Equal(t, "go1.25.3", settings.LoadedEnvVars()["GOTOOLCHAIN"])

	// No error messages should have been logged
	logOutput := buf.String()
	assert.NotContains(t, logOutput, "Not able to load")
	assert.NotContains(t, logOutput, "Not able to bind")
}

func TestLoadEnvOverridesExistingEnv(t *testing.T) {
	os.Setenv("CRE_TARGET", "production")
	t.Cleanup(func() { os.Unsetenv("CRE_TARGET") })

	tempDir := t.TempDir()
	envFilePath := filepath.Join(tempDir, ".env")
	require.NoError(t, godotenv.Write(map[string]string{
		"CRE_TARGET": "staging",
	}, envFilePath))

	logger := testutil.NewTestLogger()
	v := viper.New()
	settings.LoadEnv(logger, v, envFilePath)

	assert.Equal(t, "staging", os.Getenv("CRE_TARGET"),
		"LoadEnv should override pre-existing env vars via godotenv.Overload")
	assert.Equal(t, "staging", v.GetString("CRE_TARGET"))
}

func TestLoadEnvStateResetsBetweenCalls(t *testing.T) {
	tempDir := t.TempDir()
	envFilePath := filepath.Join(tempDir, ".env")
	require.NoError(t, godotenv.Write(map[string]string{
		"CRE_TARGET": "staging",
	}, envFilePath))

	t.Cleanup(func() { os.Unsetenv("CRE_TARGET") })

	logger := testutil.NewTestLogger()
	v := viper.New()

	settings.LoadEnv(logger, v, envFilePath)
	assert.Equal(t, envFilePath, settings.LoadedEnvFilePath())
	assert.NotNil(t, settings.LoadedEnvVars())

	// Calling with empty path resets the state
	settings.LoadEnv(logger, v, "")
	assert.Empty(t, settings.LoadedEnvFilePath(), "state should be reset on subsequent call")
	assert.Nil(t, settings.LoadedEnvVars(), "state should be reset on subsequent call")
}

func TestResolveAndLoadBothEnvFiles(t *testing.T) {
	callBoth := func(logger *zerolog.Logger, v *viper.Viper) {
		settings.ResolveAndLoadBothEnvFiles(
			logger, v,
			settings.Flags.CliEnvFile.Name, constants.DefaultEnvFileName,
			settings.Flags.CliPublicEnvFile.Name, constants.DefaultPublicEnvFileName,
		)
	}

	writeFile := func(t *testing.T, path string, vars map[string]string) {
		t.Helper()
		require.NoError(t, godotenv.Write(vars, path))
	}

	t.Run("flag unspecified file auto discovered debug log emitted", func(t *testing.T) {
		tempDir := t.TempDir()
		writeFile(t, filepath.Join(tempDir, constants.DefaultEnvFileName), map[string]string{"ENV_AD": "env-val"})
		writeFile(t, filepath.Join(tempDir, constants.DefaultPublicEnvFileName), map[string]string{"PUB_AD": "pub-val"})
		t.Cleanup(func() { os.Unsetenv("ENV_AD"); os.Unsetenv("PUB_AD") })

		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "--env not specified")
		assert.Contains(t, logOutput, "--public-env not specified")
		assert.Contains(t, logOutput, "auto-discovered")

		assert.Equal(t, "env-val", os.Getenv("ENV_AD"))
		assert.Equal(t, "pub-val", os.Getenv("PUB_AD"))
		assert.Equal(t, "env-val", v.GetString("ENV_AD"))
		assert.Equal(t, "pub-val", v.GetString("PUB_AD"))
	})

	t.Run("flag unspecified file not found debug log emitted", func(t *testing.T) {
		tempDir := t.TempDir()
		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "No environment file specified")
		assert.Contains(t, logOutput, "MUST be exported")
		assert.Empty(t, settings.LoadedEnvFilePath())
		assert.Empty(t, settings.LoadedPublicEnvFilePath())
	})

	t.Run("explicit flags no unspecified debug log", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "my.env")
		pubPath := filepath.Join(tempDir, "my.env.public")
		writeFile(t, envPath, map[string]string{"E_EXPL": "1"})
		writeFile(t, pubPath, map[string]string{"P_EXPL": "2"})
		t.Cleanup(func() { os.Unsetenv("E_EXPL"); os.Unsetenv("P_EXPL") })

		logger, buf := newBufferLogger()
		v := viper.New()
		v.Set(settings.Flags.CliEnvFile.Name, envPath)
		v.Set(settings.Flags.CliPublicEnvFile.Name, pubPath)
		callBoth(logger, v)

		logOutput := buf.String()
		assert.NotContains(t, logOutput, "not specified")
		assert.NotContains(t, logOutput, "auto-discovered")

		assert.Equal(t, "1", os.Getenv("E_EXPL"))
		assert.Equal(t, "2", os.Getenv("P_EXPL"))
		assert.Equal(t, "1", v.GetString("E_EXPL"))
		assert.Equal(t, "2", v.GetString("P_EXPL"))
	})

	t.Run("invalid file path error logged", func(t *testing.T) {
		logger, buf := newBufferLogger()
		v := viper.New()
		v.Set(settings.Flags.CliEnvFile.Name, "/nonexistent/.env")
		callBoth(logger, v)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "Not able to load configuration from environment file")
		assert.Contains(t, logOutput, "dotenvx.com/docs/env-file")
		assert.Empty(t, settings.LoadedEnvFilePath())
		assert.Nil(t, settings.LoadedEnvVars())
	})

	t.Run("public env overrides env file for same key and warns", func(t *testing.T) {
		tempDir := t.TempDir()
		writeFile(t, filepath.Join(tempDir, constants.DefaultEnvFileName), map[string]string{"PRIO_VAR": "from-env"})
		writeFile(t, filepath.Join(tempDir, constants.DefaultPublicEnvFileName), map[string]string{"PRIO_VAR": "from-public"})
		t.Cleanup(func() { os.Unsetenv("PRIO_VAR") })

		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		assert.Equal(t, "from-public", os.Getenv("PRIO_VAR"))
		assert.Equal(t, "from-public", v.GetString("PRIO_VAR"))

		logOutput := buf.String()
		assert.Contains(t, logOutput, "PRIO_VAR")
		assert.Contains(t, logOutput, "defined in both")
		assert.Contains(t, logOutput, constants.DefaultPublicEnvFileName)
	})

	t.Run("env file overrides pre existing os vars", func(t *testing.T) {
		t.Setenv("OS_VAR", "from-shell")

		tempDir := t.TempDir()
		writeFile(t, filepath.Join(tempDir, constants.DefaultEnvFileName), map[string]string{"OS_VAR": "from-env-file"})
		t.Cleanup(func() { os.Unsetenv("OS_VAR") })

		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		assert.Equal(t, "from-env-file", os.Getenv("OS_VAR"))
		assert.Equal(t, "from-env-file", v.GetString("OS_VAR"))
		assert.NotContains(t, buf.String(), "level\":\"error\"")
	})

	t.Run("no warning when keys are distinct", func(t *testing.T) {
		tempDir := t.TempDir()
		writeFile(t, filepath.Join(tempDir, constants.DefaultEnvFileName), map[string]string{"ONLY_ENV": "e"})
		writeFile(t, filepath.Join(tempDir, constants.DefaultPublicEnvFileName), map[string]string{"ONLY_PUB": "p"})
		t.Cleanup(func() { os.Unsetenv("ONLY_ENV"); os.Unsetenv("ONLY_PUB") })

		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		assert.NotContains(t, buf.String(), "defined in both")
		assert.Equal(t, "e", os.Getenv("ONLY_ENV"))
		assert.Equal(t, "p", os.Getenv("ONLY_PUB"))
	})

	t.Run("all vars from both files accessible via viper", func(t *testing.T) {
		tempDir := t.TempDir()
		writeFile(t, filepath.Join(tempDir, constants.DefaultEnvFileName), map[string]string{
			"CUSTOM_ENV_VAR":             "env-value",
			settings.EthPrivateKeyEnvVar: "abc123",
			settings.CreTargetEnvVar:     "staging",
		})
		writeFile(t, filepath.Join(tempDir, constants.DefaultPublicEnvFileName), map[string]string{
			"CUSTOM_PUB_VAR": "pub-value",
			"GOTOOLCHAIN":    "go1.25.3",
		})
		t.Cleanup(func() {
			for _, k := range []string{
				"CUSTOM_ENV_VAR", "CUSTOM_PUB_VAR", "GOTOOLCHAIN",
				settings.EthPrivateKeyEnvVar, settings.CreTargetEnvVar,
			} {
				os.Unsetenv(k)
			}
		})

		restoreWD, err := testutil.ChangeWorkingDirectory(tempDir)
		require.NoError(t, err)
		defer restoreWD()

		logger, buf := newBufferLogger()
		v := viper.New()
		callBoth(logger, v)

		assert.Equal(t, "env-value", v.GetString("CUSTOM_ENV_VAR"))
		assert.Equal(t, "abc123", v.GetString(settings.EthPrivateKeyEnvVar))
		assert.Equal(t, "staging", v.GetString(settings.CreTargetEnvVar))
		assert.Equal(t, "pub-value", v.GetString("CUSTOM_PUB_VAR"))
		assert.Equal(t, "go1.25.3", v.GetString("GOTOOLCHAIN"))
		assert.NotContains(t, buf.String(), "\"level\":\"error\"")
	})
}
