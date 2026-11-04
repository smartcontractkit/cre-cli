package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyBinaryRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts used for unix fake binaries")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cre-test")
	script := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0755))

	require.NoError(t, verifyBinaryRuns(binPath))

	failPath := filepath.Join(tmpDir, "cre-fail")
	failScript := "#!/bin/sh\necho glibc error >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(failPath, []byte(failScript), 0755))

	err := verifyBinaryRuns(failPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "glibc error")
}

func TestReplaceBinaryAt_restoresBackupOnPostInstallFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only replace flow")
	}

	tmpDir := t.TempDir()
	current := filepath.Join(tmpDir, "cre")
	newBin := filepath.Join(tmpDir, "cre-new")

	currentScript := "#!/bin/sh\necho old\n"
	require.NoError(t, os.WriteFile(current, []byte(currentScript), 0755))

	newScript := "#!/bin/sh\necho broken >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(newBin, []byte(newScript), 0755))

	err := replaceBinaryAt(current, newBin)
	require.Error(t, err)
	require.Contains(t, err.Error(), "restored previous binary")

	restored, err := os.ReadFile(current)
	require.NoError(t, err)
	require.Equal(t, currentScript, string(restored))
	require.NoFileExists(t, current+backupSuffix)
}

func TestReplaceBinaryAt_succeedsWithWorkingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only replace flow")
	}

	tmpDir := t.TempDir()
	current := filepath.Join(tmpDir, "cre")
	newBin := filepath.Join(tmpDir, "cre-new")

	currentScript := "#!/bin/sh\necho old\n"
	require.NoError(t, os.WriteFile(current, []byte(currentScript), 0755))

	newScript := "#!/bin/sh\necho new\n"
	require.NoError(t, os.WriteFile(newBin, []byte(newScript), 0755))

	require.NoError(t, replaceBinaryAt(current, newBin))

	updated, err := os.ReadFile(current)
	require.NoError(t, err)
	require.Equal(t, newScript, string(updated))
	require.NoFileExists(t, current+backupSuffix)
	require.NoFileExists(t, newBin)
}

func TestVerifyBinaryRuns_realGoBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts used for unix fake binaries")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "version-bin")
	out, err := exec.Command("go", "build", "-o", binPath, "testdata/version_main.go").CombinedOutput()
	require.NoError(t, err, string(out))

	require.NoError(t, verifyBinaryRuns(binPath))
}
