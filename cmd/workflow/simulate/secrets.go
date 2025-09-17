package simulate

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Represents the structure of the secrets.yaml file
type secretsYamlConfig struct {
	SecretsNames map[string][]string `yaml:"secretsNames"`
}

// Replace secret names with secrets values that must be loaded into the environment by the user
func ReplaceSecretNamesWithEnvVars(secrets []byte) ([]byte, error) {
	var secretsYaml secretsYamlConfig
	if err := yaml.Unmarshal(secrets, &secretsYaml); err != nil {
		return nil, err
	}

	secretsStr := string(secrets)

	for _, values := range secretsYaml.SecretsNames {
		// Replace each secret name with the corresponding environment variable
		for _, value := range values {
			envVar, exist := os.LookupEnv(value)
			if !exist {
				return nil, fmt.Errorf("environment variable %s for secret value not found, please export it to your environment", value)
			}
			secretsStr = strings.ReplaceAll(secretsStr, value, envVar)
		}
	}

	return []byte(secretsStr), nil
}
