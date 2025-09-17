package test

import (
	"fmt"
	"os"
)

func LookupAndUnsetEnvVar(envVar string) string {
	value, found := os.LookupEnv(envVar)
	if found {
		fmt.Printf("Found %s env var\n", envVar)
		err := os.Unsetenv(envVar)
		if err != nil {
			fmt.Printf("Not able to unset %s env var\n", envVar)
			return ""
		}
	}
	return value
}

func RestoreEnvVar(envVar string, value string) {
	if value == "" {
		fmt.Printf("There was nothing to restore for %s env var\n", envVar)
		return
	}
	err := os.Setenv(envVar, value)
	if err != nil {
		fmt.Printf("Not able to restore %s env var\n", envVar)
		return
	}
	fmt.Printf("Restored %s env var\n", envVar)
}
