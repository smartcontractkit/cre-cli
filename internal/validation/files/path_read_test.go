package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestHasReadAccessToPath(t *testing.T) {
	validator, err := validation.NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type TestStruct struct {
		Path string `validate:"path_read"`
	}

	t.Run("Valid file with read access", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.txt")
		_, err := os.Create(tempFile)
		require.NoError(t, err)

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation")
	})

	t.Run("Valid directory with read access", func(t *testing.T) {
		tempDir := t.TempDir()

		input := TestStruct{Path: tempDir}
		err = validator.Struct(input)
		assert.NoError(t, err, "Expected no error during validation")
	})

	t.Run("Fails validation on not existing path", func(t *testing.T) {
		input := TestStruct{Path: "nonexistent/path"}
		err := validator.Struct(input)
		assert.ErrorContains(t, err, "Path must have read access to path: nonexistent/path")
	})

	t.Run("Fails validation on a file without read permissions", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "testfile.txt")
		_, err := os.Create(tempFile)
		require.NoError(t, err)

		err = os.Chmod(tempFile, 0333) // Write and execute only
		require.NoError(t, err, "Failed to revoke read permissions from file")

		input := TestStruct{Path: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must have read access to path: "+tempFile)
	})

	t.Run("Fails validation on a directory without read permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		err := os.Chmod(tempDir, 0333) // Write and execute only
		require.NoError(t, err)

		input := TestStruct{Path: tempDir}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must have read access to path: "+tempDir)
	})

	t.Run("Fails validation on a non-string argument", func(t *testing.T) {
		type TestStruct struct {
			InvalidField int `validate:"path_read"`
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
