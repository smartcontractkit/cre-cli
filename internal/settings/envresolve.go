package settings

import (
	"fmt"
	"os"
	"regexp"
)

// envVarPattern matches ${VAR_NAME} references in strings.
var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ResolveEnvVars replaces all ${VAR_NAME} references in s with their
// corresponding environment variable values. It returns an error if any
// referenced variable is not set.
func ResolveEnvVars(s string) (string, error) {
	var resolveErr error
	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		varName := envVarPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(varName)
		if !ok {
			resolveErr = fmt.Errorf("environment variable %q referenced in URL is not set; add it to your .env file or export it in your shell", varName)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}
