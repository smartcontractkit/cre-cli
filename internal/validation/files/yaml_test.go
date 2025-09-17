package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/validation"
)

const invalidYAML = `
key: value
nested: [key, value
`

const validYAML = `
key: value
nested:
  key: value
`

func TestIsValidYAML(t *testing.T) {
	validator, err := validation.NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type TestStruct struct {
		Path string `validate:"yaml"`
	}

	t.Run("Valid YAML file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.yaml")
		err := os.WriteFile(tempFile, []byte(validYAML), 0600)
		require.NoError(t, err, "Expected no error when creating valid YAML file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation of valid YAML file")
	})

	t.Run("Passes on a valid YAML file without extension", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile")
		err := os.WriteFile(tempFile, []byte(validYAML), 0600)
		require.NoError(t, err, "Expected no error when creating valid YAML file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation of valid YAML file")
	})

	t.Run("Fails on invalid YAML file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.yaml")

		err := os.WriteFile(tempFile, []byte(invalidYAML), 0600)
		require.NoError(t, err, "Expected no error when creating invalid YAML file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid YAML file: "+tempFile)
	})

	t.Run("Passes on empty YAML file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.yaml")
		err := os.WriteFile(tempFile, []byte{}, 0600)
		require.NoError(t, err, "Expected no error when creating empty YAML file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation of empty YAML file")
	})

	t.Run("Fails validation on not existing file", func(t *testing.T) {
		input := TestStruct{Path: "nonexistent/path.yaml"}
		err := validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid YAML file: nonexistent/path.yaml")
	})

	t.Run("Fails validation on a non-string argument", func(t *testing.T) {
		type TestStruct struct {
			InvalidField int `validate:"yaml"`
		}

		input := TestStruct{InvalidField: 42}

		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "input field name is not a string", "Expected panic for non-string argument")
			} else {
				t.Error("Expected panic, but did not get one")
			}
		}()

		_ = validator.Struct(input)
	})
}
