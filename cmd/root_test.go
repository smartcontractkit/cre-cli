package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// buildCommandPath constructs a minimal cobra command hierarchy so that
// cmd.CommandPath() returns the desired space-separated path string.
// The leaf (last segment) is returned.
func buildCommandPath(path ...string) *cobra.Command {
	root := &cobra.Command{Use: path[0]}
	parent := root
	for _, seg := range path[1:] {
		child := &cobra.Command{Use: seg}
		parent.AddCommand(child)
		parent = child
	}
	return parent
}

func TestIsRegistryRPCCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    []string
		wantRPC bool
	}{
		// Workflow lifecycle — must validate RPC (on-chain or nil registry).
		{"workflow deploy", []string{"cre", "workflow", "deploy"}, true},
		{"workflow pause", []string{"cre", "workflow", "pause"}, true},
		{"workflow activate", []string{"cre", "workflow", "activate"}, true},
		{"workflow delete", []string{"cre", "workflow", "delete"}, true},
		// Account key commands — always on-chain.
		{"account link-key", []string{"cre", "account", "link-key"}, true},
		{"account unlink-key", []string{"cre", "account", "unlink-key"}, true},
		// Other commands — must NOT trigger RPC validation.
		{"workflow simulate", []string{"cre", "workflow", "simulate"}, false},
		{"secrets create", []string{"cre", "secrets", "create"}, false},
		{"secrets list", []string{"cre", "secrets", "list"}, false},
		{"root cre", []string{"cre"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := buildCommandPath(tc.path...)
			assert.Equal(t, tc.wantRPC, isRegistryRPCCommand(cmd),
				"isRegistryRPCCommand(%q)", cmd.CommandPath())
		})
	}
}
