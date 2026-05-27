package creinit

import "github.com/rs/zerolog"

func runBunInstall(logger *zerolog.Logger, dir string) error {
	return runCommand(logger, dir, "bun", "install", "--ignore-scripts")
}
