package creinit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

const SdkVersion = "v0.8.0"

func initializeGoModule(logger *zerolog.Logger, workingDirectory, moduleName string) error {
	var deps []string

	if shouldInitGoProject(workingDirectory) {
		err := runCommand(logger, workingDirectory, "go", "mod", "init", moduleName)
		if err != nil {
			return err
		}
		logger.Info().Msgf("→ Module initialized: %s", moduleName)
	}

	captureDep := func(args ...string) error {
		output, err := runCommandCaptureOutput(logger, workingDirectory, args...)
		if err != nil {
			return err
		}
		deps = append(deps, parseAddedModules(string(output))...)
		return nil
	}

	if err := captureDep("go", "get", "github.com/smartcontractkit/cre-sdk-go@"+SdkVersion); err != nil {
		return err
	}
	if err := captureDep("go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@"+SdkVersion); err != nil {
		return err
	}
	if err := captureDep("go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http@"+SdkVersion); err != nil {
		return err
	}
	if err := captureDep("go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron@"+SdkVersion); err != nil {
		return err
	}

	_ = runCommand(logger, workingDirectory, "go", "mod", "tidy")

	logger.Info().Msg("→ Dependencies installed:")
	for _, dep := range deps {
		logger.Info().Msgf("\t•\t%s", dep)
	}

	return nil
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

func runCommandCaptureOutput(logger *zerolog.Logger, dir string, args ...string) ([]byte, error) {
	logger.Debug().Msgf("Running command: %v in directory: %s", args, dir)

	// #nosec G204 -- args are internal and validated
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Err(err).Msgf("Command failed: %v\nOutput:\n%s", args, output)
		return output, err
	}

	logger.Debug().Msgf("Command succeeded: %v", args)
	return output, nil
}

func parseAddedModules(output string) []string {
	var modules []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go: added ") {
			modules = append(modules, strings.TrimPrefix(line, "go: added "))
		}
	}
	return modules
}
