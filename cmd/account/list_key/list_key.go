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
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
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

// Inputs holds resolved and validated flag values for list-key.
type Inputs struct {
	OutputFormat string
}

func resolveInputs(outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{OutputFormat: outputFormat}, nil
}

func New(runtimeCtx *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:     "list-key",
		Short:   "List workflow owners",
		Long:    "Fetches workflow owners linked to your organisation",
		Example: "cre account list-key\n  cre account list-key --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			h := NewHandler(runtimeCtx)
			return h.Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")

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

// workflowOwnerJSON is the stable JSON contract for `cre account list-key --json`.
type workflowOwnerJSON struct {
	WorkflowOwnerAddress string `json:"workflowOwnerAddress"`
	WorkflowOwnerLabel   string `json:"workflowOwnerLabel"`
	Environment          string `json:"environment"`
	VerificationStatus   string `json:"verificationStatus"`
	VerifiedAt           string `json:"verifiedAt"`
}

func toOwnersJSON(owners []WorkflowOwner) []workflowOwnerJSON {
	out := make([]workflowOwnerJSON, 0, len(owners))
	for _, o := range owners {
		out = append(out, workflowOwnerJSON{
			WorkflowOwnerAddress: o.WorkflowOwnerAddress,
			WorkflowOwnerLabel:   o.WorkflowOwnerLabel,
			Environment:          o.Environment,
			VerificationStatus:   o.VerificationStatus,
			VerifiedAt:           o.VerifiedAt,
		})
	}
	return out
}

func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
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

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintJSON(toOwnersJSON(respEnvelope.ListWorkflowOwners.LinkedOwners))
	}

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
