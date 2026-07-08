package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// IsolateCLIHome redirects CLI config writes (~/.cre) into a temp directory.
// Call in tests that run the cre binary or invoke EnsureContext/FetchAndWriteContext.
func IsolateCLIHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	PinGoCacheForTestHome(t)
	return home
}

// PinGoCacheForTestHome keeps GOPATH/GOMODCACHE outside temp HOME directories.
// Overriding HOME makes Go default GOPATH to $HOME/go; module files are read-only and break TempDir cleanup.
func PinGoCacheForTestHome(t *testing.T) {
	t.Helper()

	gopath, gomodcache := realGoCacheEnv(t)
	t.Setenv("GOPATH", gopath)
	t.Setenv("GOMODCACHE", gomodcache)
}

func realGoCacheEnv(t *testing.T) (gopath, gomodcache string) {
	t.Helper()

	realHome, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get real home dir")

	gopath = os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(realHome, "go")
	}

	gomodcache = os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		gomodcache = filepath.Join(gopath, "pkg", "mod")
	}

	return gopath, gomodcache
}

// CLIChildEnv builds subprocess env with isolated HOME for credentials and pinned Go cache paths.
func CLIChildEnv(t *testing.T, testHome string) []string {
	t.Helper()

	gopath, gomodcache := realGoCacheEnv(t)

	childEnv := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") ||
			strings.HasPrefix(entry, "USERPROFILE=") ||
			strings.HasPrefix(entry, "GOPATH=") ||
			strings.HasPrefix(entry, "GOMODCACHE=") {
			continue
		}
		childEnv = append(childEnv, entry)
	}
	childEnv = append(childEnv,
		"HOME="+testHome,
		"USERPROFILE="+testHome,
		"GOPATH="+gopath,
		"GOMODCACHE="+gomodcache,
	)
	return childEnv
}
