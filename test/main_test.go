package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

	// Run all tests
	exitCode := m.Run()

	// Restore env var that were previously present in this user's shell environment
	RestoreEnvVar(settings.EthPrivateKeyEnvVar, ethPrivateKeyValue)

	// Exit with the appropriate code
	os.Exit(exitCode)
}
