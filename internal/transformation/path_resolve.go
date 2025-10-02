package transformation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath resolves an input path to an absolute path.
// If the path exists, it returns the absolute path; otherwise, returns an error.
func ResolvePath(input string) (string, error) {

	input = filepath.Clean(input)

	// Resolve absolute path
	absPath, err := resolveAbsolutePath(input)
	if err != nil {
		return "", err
	}

	// Check if the path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", errors.New("path does not exist")
	}

	return absPath, nil
}

// ResolveDirectoryOrCreate resolves a path and ensures the directory exists.
// If the path is a file, it only creates the parent directories.
func ResolveDirectoryOrCreate(input string) (string, error) {

	input = filepath.Clean(input)

	// Resolve absolute path
	absPath, err := resolveAbsolutePath(input)
	if err != nil {
		return "", err
	}

	// If the path already exists and is a directory, return it
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		return absPath, nil
	}

	// Ensure parent directories exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}

	return absPath, nil
}

// ResolveFilePathOrCreate resolves a path, ensuring the file and its parent directories exist.
func ResolveFilePathOrCreate(input string) (string, error) {
	// Clean the input path
	input = filepath.Clean(input)

	// Resolve to absolute path
	absPath, err := resolveAbsolutePath(input)
	if err != nil {
		return "", err
	}

	// Ensure parent directories exist
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}

	// Create the file if it does not exist
	file, err := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return "", err
	}
	file.Close()

	return absPath, nil
}

// ResolveWorkflowPath resolves a workflow directory path that can be:
// - Absolute path
// - Relative path (resolved relative to current working directory, which should be the project root)
// Returns an error if the path doesn't exist or isn't a directory.
func ResolveWorkflowPath(workflowPath string) (string, error) {
	if workflowPath == "" {
		return "", errors.New("workflow path cannot be empty")
	}

	// Clean the path first
	cleanPath := filepath.Clean(workflowPath)

	// Resolve to absolute path
	absPath, err := resolveAbsolutePath(cleanPath)
	if err != nil {
		return "", err
	}

	// Check if the path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", errors.New("workflow directory does not exist")
	} else if err != nil {
		return "", err
	}

	// Check if it's a directory
	if !info.IsDir() {
		return "", errors.New("workflow path must be a directory")
	}

	return absPath, nil
}

// resolveAbsolutePath expands ~ and converts a path to an absolute path.
func resolveAbsolutePath(input string) (string, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(input, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		input = filepath.Join(homeDir, input[1:])
	}

	// Convert to absolute path
	return filepath.Abs(input)
}
