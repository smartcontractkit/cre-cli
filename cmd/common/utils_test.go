package common

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

// ResolveWorkflowDir was removed; convert uses transformation.ResolveWorkflowPath (existing function).
// Project-root behavior for convert is tested in cmd/workflow/convert/convert_test.go TestConvert_ProjectRootFlag_ResolvesWorkflowDir.

func TestResolveWorkflowPath_WorkflowDir(t *testing.T) {
	// Sanity check: ResolveWorkflowPath(workflowDir, ".") returns main.go or main.ts when present
	dir := t.TempDir()
	mainGo := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(mainGo, []byte("package main\n"), 0600))
	prev, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	absDir, _ := filepath.Abs(dir)
	got, err := ResolveWorkflowPath(absDir, ".")
	require.NoError(t, err)
	require.Equal(t, mainGo, got)
}

func TestWarnIfPrivateKeyOwner(t *testing.T) {
	// Do not use t.Parallel: stderr redirection uses package-global os.Stderr.

	captureStderr := func(f func()) string {
		t.Helper()
		old := os.Stderr
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stderr = w

		f()

		require.NoError(t, w.Close())
		os.Stderr = old

		var buf bytes.Buffer
		_, copyErr := io.Copy(&buf, r)
		require.NoError(t, copyErr)
		require.NoError(t, r.Close())
		return buf.String()
	}

	t.Run("warns for EOA owner", func(t *testing.T) {
		out := captureStderr(func() { WarnIfPrivateKeyOwner(constants.WorkflowOwnerTypeEOA, "Workflow deploy") })

		assert.Contains(t, out, "Workflow deploy is using private-key ownership")
		assert.Contains(t, out, "--unsigned")
		assert.Contains(t, out, "--changeset")
	})

	for _, ownerType := range []string{constants.WorkflowOwnerTypeMSIG, constants.WorkflowOwnerTypeOrgDerived, ""} {
		t.Run("silent for owner type "+ownerType, func(t *testing.T) {
			out := captureStderr(func() { WarnIfPrivateKeyOwner(ownerType, "Workflow deploy") })

			assert.Empty(t, strings.TrimSpace(out))
		})
	}
}
