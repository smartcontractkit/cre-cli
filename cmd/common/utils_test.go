package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
