package version_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/cmd/version"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "Release version",
			version:  "version v1.0.3-beta0",
			expected: "cre version v1.0.3-beta0",
		},
		{
			name:     "Local build hash",
			version:  "build c8ab91c87c7135aa7c57669bb454e6a3287139d7",
			expected: "cre build c8ab91c87c7135aa7c57669bb454e6a3287139d7",
		},
	}

	t.Run("Default development build", func(t *testing.T) {
		simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
		defer simulatedEnvironment.Close()

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
		cmd := version.New(ctx)

		err := cmd.Execute()
		assert.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout
		var output strings.Builder
		_, _ = io.Copy(&output, r)

		assert.Contains(t, output.String(), "development", "Output does not match for %s", "Default development build")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version.Version = tt.version

			simulatedEnvironment := chainsim.NewSimulatedEnvironment(t)
			defer simulatedEnvironment.Close()

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			ctx, _ := simulatedEnvironment.NewRuntimeContextWithBufferedOutput()
			cmd := version.New(ctx)

			err := cmd.Execute()
			assert.NoError(t, err)

			w.Close()
			os.Stdout = oldStdout
			var output strings.Builder
			_, _ = io.Copy(&output, r)

			assert.Contains(t, output.String(), tt.expected, "Output does not match for %s", tt.name)
		})
	}
}
