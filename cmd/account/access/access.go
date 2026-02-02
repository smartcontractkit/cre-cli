package access

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/accessrequest"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Check or request deployment access",
		Long:  "Check your deployment access status or request access to deploy workflows.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := NewHandler(runtimeCtx)
			return h.Execute()
		},
	}

	return cmd
}

type Handler struct {
	log         *zerolog.Logger
	credentials *credentials.Credentials
	requester   *accessrequest.Requester
}

func NewHandler(ctx *runtime.Context) *Handler {
	return &Handler{
		log:         ctx.Logger,
		credentials: ctx.Credentials,
		requester:   accessrequest.NewRequester(ctx.Credentials, ctx.Logger),
	}
}

func (h *Handler) Execute() error {
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		return fmt.Errorf("failed to check deployment access: %w", err)
	}

	if deployAccess.HasAccess {
		ui.Line()
		ui.Success("You have deployment access enabled for your organization.")
		ui.Line()
		ui.Print("You're all set to deploy workflows. Get started with:")
		ui.Line()
		ui.Command("  cre workflow deploy")
		ui.Line()
		ui.Dim("For more information, run 'cre workflow deploy --help'")
		ui.Line()
		return nil
	}

	return h.requester.PromptAndSubmitRequest()
}
