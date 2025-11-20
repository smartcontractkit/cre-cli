package list

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/profiles"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all authentication profiles",
		Long:  "Display all available authentication profiles and show which one is currently active.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx)
			return h.execute()
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

func (h *handler) execute() error {
	profileMgr, err := profiles.New(h.log)
	if err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}

	profileList := profileMgr.ListProfiles()
	if len(profileList) == 0 {
		fmt.Println("No profiles found. Run 'cre login' to create one.")
		return nil
	}

	activeProfile := profileMgr.GetActiveProfileName()

	fmt.Println("")
	fmt.Println("Available Profiles:")
	fmt.Println("==================")
	fmt.Println("")

	for _, profile := range profileList {
		marker := "  "
		if profile.Name == activeProfile {
			marker = "* "
		}

		fmt.Printf("%s%-20s", marker, profile.Name)

		if profile.Email != "" {
			fmt.Printf("  (%s)", profile.Email)
		}
		if profile.Org != "" {
			fmt.Printf("  [%s]", profile.Org)
		}

		fmt.Println()
	}

	fmt.Println("")
	fmt.Printf("Active profile: %s\n", activeProfile)
	fmt.Println("")
	fmt.Println("To switch profiles, run: cre profile use <profile-name>")

	return nil
}
