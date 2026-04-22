package creinit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/constants"
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
			"cre-sdk-go@" + constants.SdkVersion,
			"capabilities/blockchain/evm@" + constants.EVMCapabilitiesVersion,
			"capabilities/networking/http@" + constants.HTTPCapabilitiesVersion,
			"capabilities/scheduler/cron@" + constants.CronCapabilitiesVersion,
		},
	}

	if shouldInitGoProject(workingDirectory) {
		err := runCommand(logger, workingDirectory, "go", "mod", "init", moduleName)
		if err != nil {
			return nil, err
		}
	}

	// Single go get: one module graph resolution instead of four sequential
	// downloads (important for tests under tight -timeout, e.g. -timeout=120s).
	getArgs := []string{
		"get",
		"github.com/smartcontractkit/cre-sdk-go@" + constants.SdkVersion,
		"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@" + constants.EVMCapabilitiesVersion,
		"github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http@" + constants.HTTPCapabilitiesVersion,
		"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron@" + constants.CronCapabilitiesVersion,
	}
	if err := runCommand(logger, workingDirectory, "go", getArgs...); err != nil {
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
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Info().Msgf("%s", string(output))
		return err
	}

	logger.Debug().Msgf("Command succeeded: %s %v", command, args)
	return nil
}
