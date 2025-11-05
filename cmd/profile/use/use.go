package use

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/profiles"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <profile-name>",
		Short: "Switch to a different profile",
		Long:  "Set the specified profile as the active profile for subsequent commands.",
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

	if err := profileMgr.SetActiveProfile(profileName); err != nil {
		return err
	}

	profile := profileMgr.GetProfile(profileName)
	fmt.Printf("\nSuccessfully switched to profile: %s\n", profileName)
	if profile.Email != "" {
		fmt.Printf("Email: %s\n", profile.Email)
	}
	if profile.Org != "" {
		fmt.Printf("Organization: %s\n", profile.Org)
	}
	fmt.Println("")

	return nil
}
