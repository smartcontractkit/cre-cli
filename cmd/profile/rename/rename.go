package rename

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/profiles"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old-name> <new-name>",
		Short: "Rename a profile",
		Long:  "Change the name of an existing profile.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx)
			return h.execute(args[0], args[1])
		},
	}

	return cmd
}

type handler struct {
	log *zerolog.Logger
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log: ctx.Logger,
	}
}

func (h *handler) execute(oldName, newName string) error {
	profileMgr, err := profiles.New(h.log)
	if err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}

	if err := profileMgr.RenameProfile(oldName, newName); err != nil {
		return err
	}

	fmt.Printf("Successfully renamed profile '%s' to '%s'\n", oldName, newName)

	return nil
}

