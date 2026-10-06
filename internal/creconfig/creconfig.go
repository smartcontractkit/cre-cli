package creconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

const Dir = ".cre"

// Lives here rather than in tenantctx so graphqlclient can reference it without an import cycle.
const ContextFile = "context.yaml"

// DirPath returns the absolute path to the CLI config directory.
func DirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, Dir), nil
}

// EnsureDir creates the CLI config directory with 0700 permissions if missing.
func EnsureDir() (string, error) {
	dir, err := DirPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// FilePath returns the absolute path to a file directly under the CLI config directory.
func FilePath(name string) (string, error) {
	dir, err := DirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// FilePathHint returns the absolute config file path for user-facing messages,
// or a doc-style path (Dir/name) if the home directory cannot be resolved.
func FilePathHint(name string) string {
	if path, err := FilePath(name); err == nil {
		return path
	}
	return filepath.Join(Dir, name)
}

// JoinPath returns an absolute path under the CLI config directory.
func JoinPath(elem ...string) (string, error) {
	dir, err := DirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{dir}, elem...)...), nil
}

// A missing file is not an error so callers can clear caches idempotently.
func RemoveFile(name string) error {
	path, err := FilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
