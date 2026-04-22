package list

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const listWorkflowsQuery = `
query ListWorkflows($input: WorkflowsInput!) {
  workflows(input: $input) {
    data {
      name
      workflowId
      ownerAddress
      status
      workflowSource
    }
    count
  }
}
`

const workflowListPageSize = 100

// GraphQLExecutor runs a GraphQL request (implemented by graphqlclient.Client).
type GraphQLExecutor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

type workflowRow struct {
	Name           string `json:"name"`
	WorkflowID     string `json:"workflowId"`
	OwnerAddress   string `json:"ownerAddress"`
	Status         string `json:"status"`
	WorkflowSource string `json:"workflowSource"`
}

// Handler loads and prints workflows (used by the command and tests).
type Handler struct {
	credentials *credentials.Credentials
	tenantCtx   *tenantctx.EnvironmentContext
	gql         GraphQLExecutor
}

// NewHandler builds a handler with the real GraphQL client.
func NewHandler(ctx *runtime.Context) *Handler {
	return NewHandlerWithClient(ctx, nil)
}

// NewHandlerWithClient builds a handler with an optional GraphQL client (nil uses graphqlclient.New).
func NewHandlerWithClient(ctx *runtime.Context, gql GraphQLExecutor) *Handler {
	if gql == nil {
		gql = graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	}
	return &Handler{
		credentials: ctx.Credentials,
		tenantCtx:   ctx.TenantContext,
		gql:         gql,
	}
}

// Execute lists workflows, optionally filtering by registry ID from user context.
// Deleted workflows are omitted unless includeDeleted is true.
func (h *Handler) Execute(ctx context.Context, registryFilter string, includeDeleted bool) error {
	if h.tenantCtx == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}

	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	if registryFilter != "" {
		if findRegistry(h.tenantCtx.Registries, registryFilter) == nil {
			return fmt.Errorf("registry %q not found in user context; available: [%s]",
				registryFilter, availableRegistryIDs(h.tenantCtx.Registries))
		}
	}

	spinner := ui.NewSpinner()
	spinner.Start("Listing workflows...")
	rows, err := h.fetchAllWorkflows(ctx)
	spinner.Stop()
	if err != nil {
		return err
	}

	if registryFilter != "" {
		reg := findRegistry(h.tenantCtx.Registries, registryFilter)
		rows = filterRowsByRegistry(rows, reg, h.tenantCtx.Registries)
	}

	afterRegistryFilter := len(rows)
	if !includeDeleted {
		rows = omitDeleted(rows)
	}

	h.printWorkflows(rows, afterRegistryFilter, includeDeleted)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var registryID string
	var includeDeleted bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Lists workflows deployed for your organization",
		Long:    `Lists workflows across registries in your organization. Requires authentication and user context. Deleted workflows are hidden by default.`,
		Example: "cre workflow list\n  cre workflow list --registry private\n  cre workflow list --include-deleted",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return NewHandler(runtimeContext).Execute(cmd.Context(), registryID, includeDeleted)
		},
	}

	cmd.Flags().StringVar(&registryID, "registry", "", "Filter by registry ID from user context")
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include workflows in DELETED status")
	return cmd
}

func (h *Handler) fetchAllWorkflows(ctx context.Context) ([]workflowRow, error) {
	var total int
	var all []workflowRow

	for pageNum := 0; ; pageNum++ {
		req := graphql.NewRequest(listWorkflowsQuery)
		req.Var("input", map[string]any{
			"page": map[string]any{
				"number": pageNum,
				"size":   workflowListPageSize,
			},
		})

		var envelope struct {
			Workflows struct {
				Data  []workflowRow `json:"data"`
				Count int           `json:"count"`
			} `json:"workflows"`
		}

		if err := h.gql.Execute(ctx, req, &envelope); err != nil {
			return nil, fmt.Errorf("list workflows: %w", err)
		}

		if pageNum == 0 {
			total = envelope.Workflows.Count
		}

		batch := envelope.Workflows.Data
		all = append(all, batch...)

		if len(all) >= total || len(batch) == 0 {
			break
		}
	}

	return all, nil
}

// filterRowsByRegistry keeps rows whose workflowSource resolves to the given registry.
// API workflowSource values (e.g. contract:…:0x…, grpc:…) rarely equal context registry IDs
func filterRowsByRegistry(rows []workflowRow, reg *tenantctx.Registry, all []*tenantctx.Registry) []workflowRow {
	if reg == nil {
		return rows
	}
	out := make([]workflowRow, 0, len(rows))
	for _, r := range rows {
		if rowMatchesRegistry(r.WorkflowSource, reg, all) {
			out = append(out, r)
		}
	}
	return out
}

func rowMatchesRegistry(workflowSource string, reg *tenantctx.Registry, all []*tenantctx.Registry) bool {
	if workflowSource == reg.ID {
		return true
	}

	const contractPrefix = "contract:"
	if strings.HasPrefix(workflowSource, contractPrefix) {
		return contractSourceMatchesRegistry(workflowSource, reg)
	}

	if strings.HasPrefix(workflowSource, "grpc:") {
		if !registryEligibleForGrpcRows(reg) {
			return false
		}
		resolved := resolveGrpcSourceRegistry(workflowSource, all)
		return resolved != nil && resolved.ID == reg.ID
	}

	return false
}

// registryTypeOffChain mirrors how user context may store OFF_CHAIN / off_chain / off-chain.
func registryTypeOffChain(reg *tenantctx.Registry) bool {
	if reg == nil {
		return false
	}
	t := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(reg.Type), "_", "-"))
	return t == "off-chain" || strings.EqualFold(strings.TrimSpace(reg.Type), "OFF_CHAIN")
}

func hasContractAddress(reg *tenantctx.Registry) bool {
	return reg != nil && reg.Address != nil && strings.TrimSpace(*reg.Address) != ""
}

// registryEligibleForContractRows matches formatRegistryDisplay: on-chain rows need a registry
// entry with an address; many manifests omit "type" entirely.
func registryEligibleForContractRows(reg *tenantctx.Registry) bool {
	if reg == nil || !hasContractAddress(reg) {
		return false
	}
	if registryTypeOffChain(reg) {
		return false
	}
	return true
}

// registryEligibleForGrpcRows is true for off-chain registries and for legacy entries
// with no type and no on-chain address (private / grpc-only).
func registryEligibleForGrpcRows(reg *tenantctx.Registry) bool {
	if reg == nil {
		return false
	}
	if registryTypeOffChain(reg) {
		return true
	}
	if hasContractAddress(reg) {
		return false
	}
	return true
}

func contractSourceMatchesRegistry(workflowSource string, reg *tenantctx.Registry) bool {
	if !registryEligibleForContractRows(reg) {
		return false
	}
	const contractPrefix = "contract:"
	rest := strings.TrimPrefix(workflowSource, contractPrefix)
	selector, addr, ok := strings.Cut(rest, ":")
	if !ok || addr == "" {
		return false
	}
	if !addressesEqual(addr, *reg.Address) {
		return false
	}
	if reg.ChainSelector != nil && strings.TrimSpace(*reg.ChainSelector) != "" &&
		strings.TrimSpace(*reg.ChainSelector) != strings.TrimSpace(selector) {
		return false
	}
	return true
}

// resolveGrpcSourceRegistry maps API grpc workflow sources (e.g. grpc:private-grpc-registry:v1)
// to the tenant registry from user context (same rules as --registry). Returns nil if ambiguous
// or unmatched.
func resolveGrpcSourceRegistry(workflowSource string, all []*tenantctx.Registry) *tenantctx.Registry {
	if !strings.HasPrefix(workflowSource, "grpc:") {
		return nil
	}
	eligible := make([]*tenantctx.Registry, 0, len(all))
	for _, r := range all {
		if r != nil && registryEligibleForGrpcRows(r) {
			eligible = append(eligible, r)
		}
	}
	if len(eligible) == 1 {
		return eligible[0]
	}
	var match *tenantctx.Registry
	for _, r := range eligible {
		id := strings.ToLower(strings.TrimSpace(r.ID))
		if len(id) < 3 {
			continue
		}
		if strings.Contains(strings.ToLower(workflowSource), id) {
			if match != nil {
				return nil
			}
			match = r
		}
	}
	return match
}

func omitDeleted(rows []workflowRow) []workflowRow {
	out := make([]workflowRow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Status), "DELETED") {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (h *Handler) printWorkflows(rows []workflowRow, afterRegistryFilter int, includeDeleted bool) {
	ui.Line()
	if len(rows) == 0 {
		if afterRegistryFilter > 0 && !includeDeleted {
			ui.Warning("No workflows found (excluding deleted). Use --include-deleted to list them.")
		} else {
			ui.Warning("No workflows found")
		}
		ui.Line()
		return
	}

	ui.Bold("Workflows")
	ui.Line()

	for i, r := range rows {
		regCol := formatRegistryDisplay(r.WorkflowSource, h.tenantCtx.Registries)
		ui.Bold(fmt.Sprintf("%d. %s", i+1, r.Name))
		ui.Dim(fmt.Sprintf("   Workflow ID:  %s", r.WorkflowID))
		ui.Dim(fmt.Sprintf("   Owner:        %s", r.OwnerAddress))
		ui.Dim(fmt.Sprintf("   Status:       %s", r.Status))
		ui.Dim(fmt.Sprintf("   Registry:     %s", regCol))
		ui.Line()
	}
}

func formatRegistryDisplay(workflowSource string, registries []*tenantctx.Registry) string {
	byID := registryByWorkflowSource(registries)
	if reg, ok := byID[workflowSource]; ok {
		if reg.Label != "" {
			return reg.Label
		}
		return reg.ID
	}

	const contractPrefix = "contract:"
	if strings.HasPrefix(workflowSource, contractPrefix) {
		rest := strings.TrimPrefix(workflowSource, contractPrefix)
		selector, addr, ok := strings.Cut(rest, ":")
		if ok && addr != "" {
			for _, r := range registries {
				if r == nil || r.Address == nil {
					continue
				}
				if !addressesEqual(addr, *r.Address) {
					continue
				}
				if r.ChainSelector != nil && strings.TrimSpace(*r.ChainSelector) != "" &&
					strings.TrimSpace(*r.ChainSelector) != strings.TrimSpace(selector) {
					continue
				}
				if r.Label != "" {
					return r.Label
				}
				return r.ID
			}
		}
	}

	if strings.HasPrefix(workflowSource, "grpc:") {
		if reg := resolveGrpcSourceRegistry(workflowSource, registries); reg != nil {
			return reg.ID
		}
	}

	return workflowSource
}

func addressesEqual(a, b string) bool {
	return strings.EqualFold(
		strings.TrimPrefix(strings.TrimSpace(a), "0x"),
		strings.TrimPrefix(strings.TrimSpace(b), "0x"),
	)
}

func registryByWorkflowSource(registries []*tenantctx.Registry) map[string]*tenantctx.Registry {
	m := make(map[string]*tenantctx.Registry)
	for _, r := range registries {
		if r != nil {
			m[r.ID] = r
		}
	}
	return m
}

func findRegistry(registries []*tenantctx.Registry, id string) *tenantctx.Registry {
	for _, r := range registries {
		if r != nil && r.ID == id {
			return r
		}
	}
	return nil
}

func availableRegistryIDs(registries []*tenantctx.Registry) string {
	ids := make([]string, 0, len(registries))
	for _, r := range registries {
		if r != nil {
			ids = append(ids, r.ID)
		}
	}
	return strings.Join(ids, ", ")
}
