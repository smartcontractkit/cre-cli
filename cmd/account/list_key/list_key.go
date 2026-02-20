package list_key

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const queryListWorkflowOwners = `
query {
  listWorkflowOwners(filters: { linkStatus: LINKED_ONLY }) {
    linkedOwners {
      workflowOwnerAddress
      workflowOwnerLabel
      environment
      verificationStatus
      verifiedAt
      chainSelector
      contractAddress
      requestProcess
    }
    unlinkedOwners {
      workflowOwnerAddress
      workflowOwnerLabel
      environment
      verificationStatus
      verifiedAt
      chainSelector
      contractAddress
      requestProcess
    }
  }
}
`

type GraphQLExecutor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-key",
		Short: "List workflow owners",
		Long:  "Fetches workflow owners linked to your organisation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := NewHandler(runtimeCtx)
			return h.Execute(cmd.Context())
		},
	}
	return cmd
}

type Handler struct {
	log            *zerolog.Logger
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	client         GraphQLExecutor
}

func NewHandler(ctx *runtime.Context) *Handler {
	return &Handler{
		log:            ctx.Logger,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
		client:         graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger),
	}
}

type WorkflowOwner struct {
	WorkflowOwnerAddress string `json:"workflowOwnerAddress"`
	WorkflowOwnerLabel   string `json:"workflowOwnerLabel"`
	Environment          string `json:"environment"`
	VerificationStatus   string `json:"verificationStatus"`
	VerifiedAt           string `json:"verifiedAt"`
	ChainSelector        string `json:"chainSelector"`
	ContractAddress      string `json:"contractAddress"`
	RequestProcess       string `json:"requestProcess"`
}

func (h *Handler) Execute(ctx context.Context) error {
	spinner := ui.NewSpinner()
	spinner.Start("Fetching workflow owners...")

	req := graphql.NewRequest(queryListWorkflowOwners)

	var respEnvelope struct {
		ListWorkflowOwners struct {
			LinkedOwners []WorkflowOwner `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}

	if err := h.client.Execute(ctx, req, &respEnvelope); err != nil {
		spinner.Stop()
		return fmt.Errorf("fetch workflow owners failed: %w", err)
	}

	spinner.Stop()
	ui.Success("Workflow owners retrieved successfully")
	h.logOwners("Linked Owners", respEnvelope.ListWorkflowOwners.LinkedOwners)

	return nil
}

func (h *Handler) logOwners(label string, owners []WorkflowOwner) {
	ui.Line()
	if len(owners) == 0 {
		ui.Warning(fmt.Sprintf("No %s found", strings.ToLower(label)))
		return
	}

	ui.Title(label)
	ui.Line()

	for i, o := range owners {
		ui.Bold(fmt.Sprintf("%d. %s", i+1, o.WorkflowOwnerLabel))
		ui.Dim(fmt.Sprintf("   Owner Address:     %s", o.WorkflowOwnerAddress))
		ui.Dim(fmt.Sprintf("   Status:            %s", o.VerificationStatus))
		ui.Dim(fmt.Sprintf("   Verified At:       %s", o.VerifiedAt))
		ui.Dim(fmt.Sprintf("   Chain Selector:    %s", o.ChainSelector))
		ui.Dim(fmt.Sprintf("   Contract Address:  %s", o.ContractAddress))
		ui.Line()
	}
}
