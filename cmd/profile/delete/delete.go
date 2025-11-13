package delete

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/profiles"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <profile-name>",
		Short: "Delete a profile",
		Long:  "Remove a profile and its associated credentials.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx)
			return h.execute(args[0])
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

func (h *handler) execute(profileName string) error {
	profileMgr, err := profiles.New(h.log)
	if err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}

	wasActive := profileMgr.GetActiveProfileName() == profileName

	if err := profileMgr.DeleteProfile(profileName); err != nil {
		return err
	}

	fmt.Printf("Successfully deleted profile: %s\n", profileName)

	if wasActive {
		newActive := profileMgr.GetActiveProfileName()
		if newActive != "" {
			fmt.Printf("Active profile switched to: %s\n", newActive)
		} else {
			fmt.Println("No profiles remaining.")
		}
	}

	return nil
}
