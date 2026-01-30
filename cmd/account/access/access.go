package access

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Check or request deployment access",
		Long:  "Check your deployment access status or request access to deploy workflows.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := NewHandler(runtimeCtx)
			return h.Execute(cmd.Context())
		},
	}

	return cmd
}

type Handler struct {
	log         *zerolog.Logger
	credentials *credentials.Credentials
}

func NewHandler(ctx *runtime.Context) *Handler {
	return &Handler{
		log:         ctx.Logger,
		credentials: ctx.Credentials,
	}
}

func (h *Handler) Execute(ctx context.Context) error {
	// Get deployment access status
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		return fmt.Errorf("failed to check deployment access: %w", err)
	}

	if deployAccess.HasAccess {
		fmt.Println("")
		fmt.Println("You have deployment access enabled for your organization.")
		fmt.Println("")
		fmt.Println("You're all set to deploy workflows. Get started with:")
		fmt.Println("")
		fmt.Println("  cre workflow deploy")
		fmt.Println("")
		fmt.Println("For more information, run 'cre workflow deploy --help'")
		fmt.Println("")
		return nil
	}

	// User doesn't have access - submit request to Zendesk
	// TODO: Implement Zendesk request submission
	fmt.Println("")
	fmt.Println("Deployment access is not enabled for your organization.")
	fmt.Println("")
	fmt.Println("Submitting access request...")
	fmt.Println("")

	// TODO: Call Zendesk API here

	return nil
}
