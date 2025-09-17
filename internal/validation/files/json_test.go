package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestIsValidJSON(t *testing.T) {
	validator, err := validation.NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type TestStruct struct {
		Path string `validate:"json"`
	}

	t.Run("Valid JSON file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.json")
		validJSON := `{
			"key": "value",
			"nested": {
				"key": "value"
			}
		}`
		err := os.WriteFile(tempFile, []byte(validJSON), 0600)
		require.NoError(t, err, "Expected no error when creating valid JSON file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation of valid JSON file")
	})

	t.Run("Passes on a valid JSON file without extension", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile")
		validJSON := `{
			"key": "value",
			"nested": {
				"key": "value"
			}
		}`
		err := os.WriteFile(tempFile, []byte(validJSON), 0600)
		require.NoError(t, err, "Expected no error when creating valid JSON file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation of valid JSON file")
	})

	t.Run("Fails on invalid JSON file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.json")
		invalidJSON := `{
			"key": "value",
			"nested": {
				"key": "value"
			`
		err := os.WriteFile(tempFile, []byte(invalidJSON), 0600)
		require.NoError(t, err, "Expected no error when creating invalid JSON file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid JSON file: "+tempFile)
	})

	t.Run("Fails validation on empty JSON file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.json")
		err := os.WriteFile(tempFile, []byte{}, 0600)
		require.NoError(t, err, "Expected no error when creating empty JSON file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid JSON file: "+tempFile)
	})

	t.Run("Fails validation on not existing file", func(t *testing.T) {
		input := TestStruct{Path: "nonexistent/path.json"}
		err := validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid JSON file: nonexistent/path.json")
	})

	t.Run("Fails validation on a non-string argument", func(t *testing.T) {
		type TestStruct struct {
			InvalidField int `validate:"json"`
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
