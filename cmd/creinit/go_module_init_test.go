package creinit

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/dev-platform/internal/testutil"
)

func TestShouldInitGoProject_ReturnsFalseWhenGoModExists(t *testing.T) {
	tempDir := t.TempDir()
	createGoModFile(t, tempDir, "")

	shouldInit := shouldInitGoProject(tempDir)
	assert.False(t, shouldInit)
}

func TestShouldInitGoProject_ReturnsTrueWhenThereIsOnlyGoSum(t *testing.T) {
	tempDir := t.TempDir()
	createGoSumFile(t, tempDir, "")

	shouldInit := shouldInitGoProject(tempDir)
	assert.True(t, shouldInit)
}

func TestShouldInitGoProject_ReturnsTrueInEmptyProject(t *testing.T) {
	tempDir := t.TempDir()

	shouldInit := shouldInitGoProject(tempDir)
	assert.True(t, shouldInit)
}

func TestInitializeGoModule_InEmptyProject(t *testing.T) {
	logger := testutil.NewTestLogger()

	tempDir := prepareTempDirWithMainFile(t)
	moduleName := "testmodule"

	err := initializeGoModule(logger, tempDir, moduleName)
	assert.NoError(t, err)

	// Check go.mod file was generated
	goModFilePath := filepath.Join(tempDir, "go.mod")
	_, err = os.Stat(goModFilePath)
	assert.NoError(t, err)

	goModContent, err := os.ReadFile(goModFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(goModContent), "module "+moduleName)

	// Check go.sum file was generated
	goSumFilePath := filepath.Join(tempDir, "go.sum")
	_, err = os.Stat(goSumFilePath)
	assert.NoError(t, err)

	goSumContent, err := os.ReadFile(goSumFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(goSumContent), "github.com/ethereum/go-ethereum")
}

func TestInitializeGoModule_InExistingProject(t *testing.T) {
	logger := testutil.NewTestLogger()

	tempDir := prepareTempDirWithMainFile(t)
	moduleName := "testmodule"

	goModFilePath := createGoModFile(t, tempDir, "module oldmodule")

	err := initializeGoModule(logger, tempDir, moduleName)
	assert.NoError(t, err)

	// Check go.mod file was not changed
	_, err = os.Stat(goModFilePath)
	assert.NoError(t, err)

	goModContent, err := os.ReadFile(goModFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(goModContent), "module oldmodule")

	// Check go.sum file was generated
	goSumFilePath := filepath.Join(tempDir, "go.sum")
	_, err = os.Stat(goSumFilePath)
	assert.NoError(t, err)

	// Check go.sum contains the expected dependency
	goSumContent, err := os.ReadFile(goSumFilePath)
	assert.NoError(t, err)
	assert.Contains(t, string(goSumContent), "github.com/ethereum/go-ethereum")
}

func TestInitializeGoModule_GoModInitFails(t *testing.T) {
	logger := testutil.NewTestLogger()

	tempDir := t.TempDir()
	moduleName := "testmodule"

	// Remove write access so that go mod init fails
	err := os.Chmod(tempDir, 0500) // Read and execute permissions only
	assert.NoError(t, err)

	// Attempt to initialize Go module
	err = initializeGoModule(logger, tempDir, moduleName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")

	// Ensure go.mod is not created
	goModFilePath := filepath.Join(tempDir, "go.mod")
	_, statErr := os.Stat(goModFilePath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func prepareTempDirWithMainFile(t *testing.T) string {
	tempDir := t.TempDir()

	srcFilePath := "testdata/main.go"
	destFilePath := filepath.Join(tempDir, "main.go")
	err := copyFile(srcFilePath, destFilePath)
	assert.NoError(t, err)

	return tempDir
}

func createGoModFile(t *testing.T, tempDir string, fileContent string) string {
	goModFilePath := filepath.Join(tempDir, "go.mod")
	return createFile(t, goModFilePath, fileContent)
}

func createGoSumFile(t *testing.T, tempDir string, fileContent string) string {
	goSumFilePath := filepath.Join(tempDir, "go.sum")
	return createFile(t, goSumFilePath, fileContent)
}

func createFile(t *testing.T, filePath, fileContent string) string {
	err := os.WriteFile(filePath, []byte(fileContent), 0600)
	assert.NoError(t, err)
	return filePath
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
