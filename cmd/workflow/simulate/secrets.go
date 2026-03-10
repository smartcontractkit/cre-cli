package simulate

import (
	"fmt"
	"os"
	"strings"

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

// FilterSecretsByAllowedKeys restricts the resolved secrets YAML to only the
// keys declared via --secret flags. Returns an error if a declared key is not
// present in the secrets file.
func FilterSecretsByAllowedKeys(secrets []byte, allowedSecrets []string) ([]byte, error) {
	var cfg secretsYamlConfig
	if err := yaml.Unmarshal(secrets, &cfg); err != nil {
		return nil, err
	}

	allowed := make(map[string]bool, len(allowedSecrets))
	for _, s := range allowedSecrets {
		key, _, _ := strings.Cut(s, ":")
		allowed[key] = true
	}

	// Verify all declared keys exist in the secrets file.
	for key := range allowed {
		if _, ok := cfg.SecretsNames[key]; !ok {
			return nil, fmt.Errorf("declared secret %q not found in secrets.yaml", key)
		}
	}

	filtered := make(map[string][]string, len(allowed))
	for key, vals := range cfg.SecretsNames {
		if allowed[key] {
			filtered[key] = vals
		}
	}

	out, err := yaml.Marshal(secretsYamlConfig{SecretsNames: filtered})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal filtered secrets: %w", err)
	}
	return out, nil
}

// secretKeys extracts just the key portion from "KEY:namespace" entries.
func secretKeys(secrets []string) []string {
	keys := make([]string, len(secrets))
	for i, s := range secrets {
		keys[i], _, _ = strings.Cut(s, ":")
	}
	return keys
}
