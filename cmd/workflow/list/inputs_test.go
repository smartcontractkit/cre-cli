package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInputs_OutputFormat_Empty(t *testing.T) {
	inputs, err := resolveInputs("", false, "", false)
	require.NoError(t, err)
	assert.Equal(t, "", inputs.OutputFormat)
}

func TestResolveInputs_OutputFormat_JSON(t *testing.T) {
	inputs, err := resolveInputs("", false, "json", false)
	require.NoError(t, err)
	assert.Equal(t, "json", inputs.OutputFormat)
}

func TestResolveInputs_OutputFormat_JSONShorthand(t *testing.T) {
	inputs, err := resolveInputs("", false, "", true)
	require.NoError(t, err)
	assert.Equal(t, "json", inputs.OutputFormat)
}

func TestResolveInputs_OutputFormat_RejectsUnsupported(t *testing.T) {
	cases := []string{"csv", "yaml", "table", "text", "JSON", "Json", "/path/to/file.json"}
	for _, format := range cases {
		t.Run(format, func(t *testing.T) {
			_, err := resolveInputs("", false, format, false)
			require.Error(t, err, "expected error for unsupported format %q", format)
			assert.Contains(t, err.Error(), "json")
		})
	}
}

func TestResolveInputs_PassthroughFields(t *testing.T) {
	inputs, err := resolveInputs("private", true, "", false)
	require.NoError(t, err)
	assert.Equal(t, "private", inputs.RegistryFilter)
	assert.True(t, inputs.IncludeDeleted)
	assert.Equal(t, "", inputs.OutputFormat)
}
