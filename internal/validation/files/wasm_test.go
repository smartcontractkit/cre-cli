package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func TestIsValidWASM(t *testing.T) {
	validator, err := validation.NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type TestStruct struct {
		FilePath string `validate:"wasm"`
	}

	t.Run("Valid WASM file", func(t *testing.T) {
		tempDir := t.TempDir()

		validWASMPath := filepath.Join(tempDir, "valid.wasm")
		validWASMContent := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
		err := os.WriteFile(validWASMPath, validWASMContent, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: validWASMPath}
		err = validator.Struct(input)
		assert.NoError(t, err)
	})

	t.Run("Valid WASM file (skips version number validation)", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "valid.wasm")
		validWASMContent := []byte{0x00, 0x61, 0x73, 0x6D, 0x02, 0x00, 0x00, 0x00}
		err := os.WriteFile(tempFile, validWASMContent, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err)
	})

	t.Run("Valid Brotli-compressed WASM file", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "valid.wasm.br")
		validWASMContent := []byte{0x00, 0x61, 0x73, 0x6D, 0x02, 0x00, 0x00, 0x00}
		file, err := os.Create(tempFile)
		require.NoError(t, err)

		// Write the file using Brotli compression
		writer := brotli.NewWriter(file)
		_, err = writer.Write(validWASMContent)
		require.NoError(t, err)
		writer.Close()
		file.Close()

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.NoError(t, err)
	})

	t.Run("Invalid WASM file (wrong file extension)", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "valid_wasm.txt")
		validWASMContent := []byte{0x00, 0x61, 0x73, 0x6D, 0x02, 0x00, 0x00, 0x00}
		err := os.WriteFile(tempFile, validWASMContent, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempFile)
	})

	t.Run("Invalid WASM file with Brotli compression (missing .wasm in the extension)", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "valid.br")
		validWASMContent := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
		file, err := os.Create(tempFile)
		require.NoError(t, err)

		// Write the file using Brotli compression
		writer := brotli.NewWriter(file)
		_, err = writer.Write(validWASMContent)
		require.NoError(t, err)
		writer.Close()
		file.Close()

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempFile)
	})

	t.Run("Invalid WASM file (wrong magic number)", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "invalid.wasm")
		invalidWASMContent := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00}
		err := os.WriteFile(tempFile, invalidWASMContent, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempFile)
	})

	t.Run("Broken Brotli-compressed WASM file", func(t *testing.T) {
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "broken.wasm.br")

		// Write invalid Brotli content (not actual Brotli-compressed data)
		invalidBrotliContent := []byte{0x01, 0x02, 0x03, 0x04}
		err := os.WriteFile(tempFile, invalidBrotliContent, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempFile)

	})

	t.Run("Invalid WASM file (empty file)", func(t *testing.T) {
		tempDir := t.TempDir()

		tempFile := filepath.Join(tempDir, "empty.wasm")
		err := os.WriteFile(tempFile, []byte{}, 0600)
		require.NoError(t, err)

		input := TestStruct{FilePath: tempFile}
		err = validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempFile)
	})

	t.Run("File does not exist", func(t *testing.T) {
		input := TestStruct{FilePath: "nonexistent.wasm"}
		err := validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: nonexistent.wasm")
	})

	t.Run("Path is a directory", func(t *testing.T) {
		tempDir := t.TempDir()

		input := TestStruct{FilePath: tempDir}
		err := validator.Struct(input)
		assert.ErrorContains(t, err, "Path must be a valid WASM file: "+tempDir)
	})

	t.Run("Fails validation on a non-string argument", func(t *testing.T) {
		type TestStruct struct {
			InvalidField int `validate:"wasm"`
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
