package creinit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
)

const (
	SdkVersion              = "v1.1.4"
	EVMCapabilitiesVersion  = "v1.0.0-beta.3"
	HTTPCapabilitiesVersion = "v1.0.0-beta.0"
	CronCapabilitiesVersion = "v1.0.0-beta.0"
)

// InstalledDependencies contains info about installed Go dependencies
type InstalledDependencies struct {
	ModuleName string
	Deps       []string
}

func initializeGoModule(logger *zerolog.Logger, workingDirectory, moduleName string) (*InstalledDependencies, error) {
	result := &InstalledDependencies{
		ModuleName: moduleName,
		Deps: []string{
			"cre-sdk-go@" + SdkVersion,
			"capabilities/blockchain/evm@" + EVMCapabilitiesVersion,
			"capabilities/networking/http@" + HTTPCapabilitiesVersion,
			"capabilities/scheduler/cron@" + CronCapabilitiesVersion,
		},
	}

	if shouldInitGoProject(workingDirectory) {
		err := runCommand(logger, workingDirectory, "go", "mod", "init", moduleName)
		if err != nil {
			return nil, err
		}
	}

	if err := runCommand(logger, workingDirectory, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+SdkVersion); err != nil {
		return nil, err
	}
	if err := runCommand(logger, workingDirectory, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@"+EVMCapabilitiesVersion); err != nil {
		return nil, err
	}
	if err := runCommand(logger, workingDirectory, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http@"+HTTPCapabilitiesVersion); err != nil {
		return nil, err
	}
	if err := runCommand(logger, workingDirectory, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron@"+CronCapabilitiesVersion); err != nil {
		return nil, err
	}

	_ = runCommand(logger, workingDirectory, "go", "mod", "tidy")

	return result, nil
}

func shouldInitGoProject(directory string) bool {
	filePath := filepath.Join(directory, "go.mod")
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return true
	}

	return false
}

func runCommand(logger *zerolog.Logger, dir, command string, args ...string) error {
	logger.Debug().Msgf("Running command: %s %v in directory: %s", command, args, dir)

	cmd := exec.Command(command, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Msgf("Command failed: %s %v\nOutput:\n%s", command, args, output)
		return err
	}

	logger.Debug().Msgf("Command succeeded: %s %v", command, args)
	return nil
}
