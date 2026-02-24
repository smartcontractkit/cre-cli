package simulate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestReplaceSecretNamesWithEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		yamlInput   string
		envVars     map[string]string
		wantSecrets map[string][]string
		wantErr     string
	}{
		{
			name: "basic replacement",
			yamlInput: `secretsNames:
  API_KEY:
    - CRE_API_KEY`,
			envVars:     map[string]string{"CRE_API_KEY": "super-secret"},
			wantSecrets: map[string][]string{"API_KEY": {"super-secret"}},
		},
		{
			name: "env var name is substring of secret name — no corruption",
			yamlInput: `secretsNames:
  MY_API_KEY:
    - API_KEY`,
			envVars:     map[string]string{"API_KEY": "the-value"},
			wantSecrets: map[string][]string{"MY_API_KEY": {"the-value"}},
		},
		{
			name: "secret name equals env var name — still works, just warns",
			yamlInput: `secretsNames:
  API_KEY:
    - API_KEY`,
			envVars:     map[string]string{"API_KEY": "actual-value"},
			wantSecrets: map[string][]string{"API_KEY": {"actual-value"}},
		},
		{
			name: "multiple secrets with overlapping names",
			yamlInput: `secretsNames:
  KEY:
    - ENV_KEY
  MY_KEY:
    - ENV_MY_KEY`,
			envVars: map[string]string{
				"ENV_KEY":    "val1",
				"ENV_MY_KEY": "val2",
			},
			wantSecrets: map[string][]string{
				"KEY":    {"val1"},
				"MY_KEY": {"val2"},
			},
		},
		{
			name: "env var not set",
			yamlInput: `secretsNames:
  SECRET:
    - MISSING_VAR`,
			envVars: map[string]string{},
			wantErr: "environment variable MISSING_VAR for secret value not found",
		},
		{
			name:      "invalid yaml",
			yamlInput: `not: valid: yaml: [`,
			envVars:   map[string]string{},
			wantErr:   "yaml:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := ReplaceSecretNamesWithEnvVars([]byte(tt.yamlInput))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)

			var parsed secretsYamlConfig
			require.NoError(t, yaml.Unmarshal(got, &parsed))
			assert.Equal(t, tt.wantSecrets, parsed.SecretsNames)
		})
	}
}
