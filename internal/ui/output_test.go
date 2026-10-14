package ui

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStreams(t *testing.T, fn func()) (stdoutOut, stderrOut string) {
	t.Helper()
	rOut, wOut, err := os.Pipe()
	require.NoError(t, err)
	rErr, wErr, err := os.Pipe()
	require.NoError(t, err)

	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr
	fn()
	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr

	var outBuf, errBuf strings.Builder
	_, _ = io.Copy(&outBuf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	return outBuf.String(), errBuf.String()
}

func printAll() {
	Line()
	Title("title")
	Success("success")
	Dim("dim")
	Print("print")
	Printf("printf %d\n", 1)
}

func TestOutputHelpers_WriteToStdoutByDefault(t *testing.T) {
	stdoutOut, stderrOut := captureStreams(t, printAll)

	for _, want := range []string{"title", "success", "dim", "print", "printf 1"} {
		assert.Contains(t, stdoutOut, want)
	}
	assert.Empty(t, stderrOut)
}

func TestRedirectStdoutToStderr(t *testing.T) {
	var restore func()
	stdoutOut, stderrOut := captureStreams(t, func() {
		restore = RedirectStdoutToStderr()
		printAll()
	})

	assert.Empty(t, stdoutOut)
	for _, want := range []string{"title", "success", "dim", "print", "printf 1"} {
		assert.Contains(t, stderrOut, want)
	}

	restore()
	stdoutOut, stderrOut = captureStreams(t, func() { Print("after restore") })
	assert.Contains(t, stdoutOut, "after restore")
	assert.Empty(t, stderrOut)
}

func TestRedirectStdoutToStderr_NestedRestoreKeepsOuterRedirect(t *testing.T) {
	outer := RedirectStdoutToStderr()
	defer outer()

	inner := RedirectStdoutToStderr()
	inner()

	stdoutOut, stderrOut := captureStreams(t, func() { Print("still redirected") })
	assert.Empty(t, stdoutOut)
	assert.Contains(t, stderrOut, "still redirected")
}
