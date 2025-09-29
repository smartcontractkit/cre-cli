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
	req := graphql.NewRequest(queryListWorkflowOwners)

	var respEnvelope struct {
		ListWorkflowOwners struct {
			LinkedOwners []WorkflowOwner `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}

	if err := h.client.Execute(ctx, req, &respEnvelope); err != nil {
		return fmt.Errorf("fetch workflow owners failed: %w", err)
	}

	h.log.Info().Msg("\nWorkflow owners retrieved successfully:")
	h.logOwners("Linked Owners", respEnvelope.ListWorkflowOwners.LinkedOwners)

	return nil
}

func (h *Handler) logOwners(label string, owners []WorkflowOwner) {
	h.log.Info().Msg("")
	if len(owners) == 0 {
		h.log.Info().Msgf("  No %s found", strings.ToLower(label))
		return
	}

	h.log.Info().Msgf("%s:", label)
	h.log.Info().Msg("")

	for i, o := range owners {
		h.log.Info().Msgf("  %d. %s", i+1, o.WorkflowOwnerLabel)
		h.log.Info().Msgf("     Owner Address:    	%s", o.WorkflowOwnerAddress)
		h.log.Info().Msgf("     Status:           	%s", o.VerificationStatus)
		h.log.Info().Msgf("     Verified At:      	%s", o.VerifiedAt)
		h.log.Info().Msgf("     Chain Selector:   	%s", o.ChainSelector)
		h.log.Info().Msgf("     Contract Address: 	%s", o.ContractAddress)
		h.log.Info().Msg("")
	}
}
