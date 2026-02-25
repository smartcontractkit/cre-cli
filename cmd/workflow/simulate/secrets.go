package simulate

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// Represents the structure of the secrets.yaml file
type secretsYamlConfig struct {
	SecretsNames map[string][]string `yaml:"secretsNames"`
}

// ReplaceSecretNamesWithEnvVars resolves env var references in the secrets YAML,
// returning a new YAML with the env var names replaced by their actual values.
// It rebuilds the YAML from the parsed structure to avoid substring corruption.
func ReplaceSecretNamesWithEnvVars(secrets []byte) ([]byte, error) {
	var secretsYaml secretsYamlConfig
	if err := yaml.Unmarshal(secrets, &secretsYaml); err != nil {
		return nil, err
	}

	resolved := make(map[string][]string, len(secretsYaml.SecretsNames))

	for secretName, values := range secretsYaml.SecretsNames {
		resolvedValues := make([]string, 0, len(values))
		for _, envVarName := range values {
			if envVarName == secretName {
				ui.Warning(fmt.Sprintf(
					"Secret %q uses itself as the env var name — this is fragile and may cause confusion. "+
						"Consider using a distinct env var name (e.g. %q).",
					secretName, "CRE_"+secretName,
				))
			}

			envVal, exist := os.LookupEnv(envVarName)
			if !exist {
				return nil, fmt.Errorf("environment variable %s for secret value not found, please export it to your environment", envVarName)
			}
			resolvedValues = append(resolvedValues, envVal)
		}
		resolved[secretName] = resolvedValues
	}

	out, err := yaml.Marshal(secretsYamlConfig{SecretsNames: resolved})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resolved secrets: %w", err)
	}
	return out, nil
}
