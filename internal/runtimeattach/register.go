// Package runtimeattach maps each Cobra command to a runtime.Context attach
// spec (which credentials, settings, and registry/owner resolution to load).
// Register from each command’s New() so the attach plan lives next to the
// command, not in a global path table.
package runtimeattach

import (
	"sync"

	"github.com/spf13/cobra"

	creruntime "github.com/smartcontractkit/cre-cli/internal/runtime"
)

// Register sets the runtime attach spec for a specific Cobra command. Call
// from each command's New() (and for each subcommand) so the decision of what
// to attach is defined next to the command, not in a global path table.
// Nil spec is treated as empty (no attach work in Apply).
func Register(cmd *cobra.Command, spec *creruntime.AttachConfig) {
	if cmd == nil {
		return
	}
	if spec == nil {
		spec = Empty
	}
	mu.Lock()
	defer mu.Unlock()
	registeredByCommand[cmd] = spec
}

// SpecForCommand returns the attach spec for the Cobra command that is
// actually running (the leaf the user invoked). Commands that were not
// registered (e.g. shell completion) get an empty spec.
func SpecForCommand(cmd *cobra.Command) *creruntime.AttachConfig {
	if cmd == nil {
		return Empty
	}
	mu.RLock()
	defer mu.RUnlock()
	if s, ok := registeredByCommand[cmd]; ok {
		return s
	}
	return Empty
}

var (
	mu                  sync.RWMutex
	registeredByCommand = make(map[*cobra.Command]*creruntime.AttachConfig)
)
