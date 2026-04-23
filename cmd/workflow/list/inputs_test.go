package list

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInputs_OutputPath_Empty(t *testing.T) {
	inputs, err := resolveInputs("", false, "")
	require.NoError(t, err)
	assert.Equal(t, "", inputs.OutputPath)
}

func TestResolveInputs_OutputPath_RejectsNonJSONExtension(t *testing.T) {
	cases := []struct {
		path string
	}{
		{"workflows.csv"},
		{"workflows.txt"},
		{"workflows"},
		{"workflows.JSON.bak"},
		{"/abs/path/to/output.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			_, err := resolveInputs("", false, tc.path)
			require.Error(t, err, "expected error for non-.json path %q", tc.path)
			assert.Contains(t, err.Error(), ".json")
		})
	}
}

func TestResolveInputs_OutputPath_AcceptsJSONExtension(t *testing.T) {
	cases := []string{
		"workflows.json",
		"output.JSON", // case-insensitive
		"./relative/path/results.json",
		"/absolute/path/workflows.json",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			inputs, err := resolveInputs("", false, tc)
			require.NoError(t, err, "expected no error for valid path %q", tc)
			assert.True(t, filepath.IsAbs(inputs.OutputPath),
				"output path should be absolute, got %q", inputs.OutputPath)
			assert.True(t, strings.HasSuffix(strings.ToLower(inputs.OutputPath), ".json"),
				"output path should end with .json, got %q", inputs.OutputPath)
		})
	}
}

func TestResolveInputs_OutputPath_RelativeBecomesAbsolute(t *testing.T) {
	inputs, err := resolveInputs("", false, "workflows.json")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(inputs.OutputPath),
		"relative path should be resolved to absolute, got %q", inputs.OutputPath)
	assert.True(t, strings.HasSuffix(inputs.OutputPath, "workflows.json"))
}

func TestResolveInputs_OutputPath_AbsolutePathUnchanged(t *testing.T) {
	abs := "/tmp/my-workflows.json"
	inputs, err := resolveInputs("", false, abs)
	require.NoError(t, err)
	assert.Equal(t, abs, inputs.OutputPath)
}

func TestResolveInputs_PassthroughFields(t *testing.T) {
	inputs, err := resolveInputs("private", true, "")
	require.NoError(t, err)
	assert.Equal(t, "private", inputs.RegistryFilter)
	assert.True(t, inputs.IncludeDeleted)
	assert.Equal(t, "", inputs.OutputPath)
}
