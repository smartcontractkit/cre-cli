package testutil

import (
	"fmt"
	"os"
)

func ChangeWorkingDirectory(newWorkingDirectory string) (func(), error) {
	origDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(newWorkingDirectory)
	if err != nil {
		return nil, err
	}

	return func() {
		err := os.Chdir(origDir)
		if err != nil {
			panic(fmt.Sprintf("failed to restore original directory: %v", err))
		}
	}, nil
}
