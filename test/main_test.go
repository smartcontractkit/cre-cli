package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// buildBinary builds the Go binary from the specified source file.
func buildBinary(sourceFile, outputBinary string) error {
	// TODO (DEVSVCS-2016) clean the conflictPolicy=ignore flag
	command := exec.Command("go", "build", "-ldflags", "-w -X 'github.com/smartcontractkit/cre-cli/cmd/version.Version=build $(git rev-parse HEAD)' -X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore", "-o", outputBinary, sourceFile)
	if os.Getenv("GOOS") == "windows" {
		command.Env = append(os.Environ(),
			"CC=gcc.exe",
			"GOARCH=amd64",
			"CGO_ENABLED=1",
		)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		fmt.Printf("Build cre binary output: %s", string(output))
		return err
	}
	return nil
}

func prepareBinary() error {
	// Build the binary before running any test
	err := buildBinary(filepath.Join(".."), CLIPath) // HL
	if err != nil {
		return err
	}
	return nil
}

func TestMain(m *testing.M) {
	// Make sure to prepare the binary before running any tests
	// If any issues appear, TestMain should catch it
	err := prepareBinary()
	if err != nil {
		fmt.Printf("Error while preparing binary: %s", err.Error())
		os.Exit(1)
	}

	// Store contents of env vars found in the shell environment and unset them
	// That way they won't leak into tests
	ethPrivateKeyValue := LookupAndUnsetEnvVar(settings.EthPrivateKeyEnvVar)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current working directory: %s\n", err.Error())
	}

	err = buildTestProjectSettings(cwd)
	if err != nil {
		fmt.Printf("Error setting up project settings: %s", err.Error())
		os.Exit(1)
	}

	// Run all tests
	exitCode := m.Run()

	// Restore env var that were previously present in this user's shell environment
	RestoreEnvVar(settings.EthPrivateKeyEnvVar, ethPrivateKeyValue)

	// Cleanup: Delete the test files
	cleanupTestFiles(cwd)
	// Exit with the appropriate code
	os.Exit(exitCode)
}

func cleanupTestFiles(cwd string) {
	// Cleanup: Delete the test project settings if it exists.
	projectSettingsPath := filepath.Join(cwd, constants.DefaultProjectSettingsFileName)
	if err := os.Remove(projectSettingsPath); err == nil {
		fmt.Printf("Removed test project settings file: %s\n", projectSettingsPath)
	}

	// Cleanup: Delete the .gitignore file if it exists.
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if err := os.Remove(gitignorePath); err == nil {
		fmt.Printf("Removed .gitignore file: %s\n", gitignorePath)
	}

	// Cleanup: Delete the .env file if it exists.
	envPath := filepath.Join(cwd, constants.DefaultEnvFileName)
	if err := os.Remove(envPath); err == nil {
		fmt.Printf("Removed environment file: %s\n", envPath)
	}
}

func buildTestProjectSettings(cwd string) error {
	replacements := settings.GetDefaultReplacements()
	replacements["WorkflowOwnerAddress"] = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	replacements["WorkflowName"] = "TestCLIWorkflowNewForCronAPIFetchMultiVariable"

	err := settings.FindOrCreateProjectSettings(cwd, replacements)
	if err != nil {
		return err
	}

	return nil
}
