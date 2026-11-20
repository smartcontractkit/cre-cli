package quotas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/registryquotaclient"
	"github.com/smartcontractkit/cre-cli/internal/registrytype"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "quotas [registry-id]",
		Short: "Shows workflow and linked-owner usage against your registry quotas",
		Long: `Shows, for each workflow registry available to your organization, how many
workflows are active against the limit of each DON family, and how many owner
addresses are linked against the linked-owner limit.

Pass a registry ID from ` + "`cre registry list`" + ` to show a single registry.

Pending link requests count against the linked-owner limit until they expire.`,
		Example: `  cre registry quotas
  cre registry quotas private
  cre registry quotas --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			if runtimeContext.TenantContext == nil {
				return fmt.Errorf("user context not available — run `cre login` and retry")
			}
			registryID := ""
			if len(args) == 1 {
				registryID = args[0]
			}

			h := &Handler{
				client:        graphqlclient.New(runtimeContext.Credentials, runtimeContext.EnvironmentSet, runtimeContext.Logger),
				tenantContext: runtimeContext.TenantContext,
			}
			return h.Execute(cmd.Context(), registryID, format)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints the quotas as JSON to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

// Handler fetches registry quotas and renders them against the registries of the
// cached user context.
type Handler struct {
	client        registryquotaclient.Executor
	tenantContext *tenantctx.EnvironmentContext
}

// NewHandler creates a Handler; it is used by tests to inject a GraphQL client.
func NewHandler(client registryquotaclient.Executor, tenantContext *tenantctx.EnvironmentContext) *Handler {
	return &Handler{client: client, tenantContext: tenantContext}
}

func (h *Handler) Execute(ctx context.Context, registryID, format string) error {
	if registryID != "" && findRegistry(h.tenantContext.Registries, registryID) == nil {
		return fmt.Errorf("unknown registry %q; valid IDs: %s", registryID, strings.Join(registryIDs(h.tenantContext.Registries), ", "))
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching registry quotas...")
	quotas, err := registryquotaclient.Fetch(ctx, h.client)
	spinner.Stop()

	// A partial result is rendered with unreadable parts shown as unavailable; any
	// other error means there is nothing to show.
	var partial *registryquotaclient.PartialError
	if err != nil && !errors.As(err, &partial) {
		return err
	}

	report := buildReport(h.tenantContext.Registries, quotas, registryID)
	if partial != nil {
		report.Warnings = append(report.Warnings, partial.Error())
	}

	if format == workflowresolve.OutputFormatJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	printReport(report)
	return nil
}

// Report is the JSON output of the command.
type Report struct {
	Registries []RegistryReport `json:"registries"`
	// LinkedOwners covers the whole organization across on-chain registries. Absent
	// when no on-chain registry is shown; null when it could not be read.
	LinkedOwners    *registryquotaclient.LinkedOwners `json:"linkedOwners"`
	hasLinkedOwners bool
	Warnings        []string `json:"warnings,omitempty"`
}

// MarshalJSON omits linkedOwners when it does not apply, and keeps it as null when it
// applies but could not be read.
func (r Report) MarshalJSON() ([]byte, error) {
	type alias Report
	if r.hasLinkedOwners {
		return json.Marshal(alias(r))
	}
	return json.Marshal(struct {
		Registries []RegistryReport `json:"registries"`
		Warnings   []string         `json:"warnings,omitempty"`
	}{r.Registries, r.Warnings})
}

// RegistryReport is one registry with its quota. Workflows is set for off-chain
// registries and Owners for on-chain ones; either is null when it could not be read.
type RegistryReport struct {
	ID            string                                `json:"id"`
	Label         string                                `json:"label"`
	Type          string                                `json:"type"`
	ChainSelector string                                `json:"chainSelector,omitempty"`
	Address       string                                `json:"address,omitempty"`
	Workflows     *[]registryquotaclient.DonFamilyQuota `json:"workflows,omitempty"`
	Owners        *[]registryquotaclient.OwnerQuota     `json:"owners,omitempty"`
	// InUserContext is false for a registry the API returned that the cached user
	// context does not know about yet.
	InUserContext bool `json:"-"`
}

func buildReport(registries []*tenantctx.Registry, quotas *registryquotaclient.Quotas, registryID string) Report {
	report := Report{Registries: []RegistryReport{}}
	usedOnChain := make([]bool, len(quotas.OnChainRegistries))
	offChainIdx := 0

	for _, reg := range registries {
		entry := RegistryReport{ID: reg.ID, Label: reg.Label, Type: reg.Type, InUserContext: true}
		if reg.ChainSelector != nil {
			entry.ChainSelector = *reg.ChainSelector
		}
		if reg.Address != nil {
			entry.Address = *reg.Address
		}

		switch registrytype.Type(reg.Type) {
		case registrytype.OffChain:
			if offChainIdx >= len(quotas.OffChainRegistries) {
				continue
			}
			workflows := quotas.OffChainRegistries[offChainIdx].Workflows
			offChainIdx++
			entry.Workflows = &workflows
		case registrytype.OnChain:
			i := matchOnChain(quotas.OnChainRegistries, entry.ChainSelector, entry.Address)
			if i < 0 {
				continue
			}
			usedOnChain[i] = true
			owners := quotas.OnChainRegistries[i].Owners
			entry.Owners = &owners
		default:
			continue
		}

		if registryID == "" || registryID == reg.ID {
			report.Registries = append(report.Registries, entry)
		}
	}

	// On-chain registries the cached user context does not know about yet.
	if registryID == "" {
		for i, q := range quotas.OnChainRegistries {
			if usedOnChain[i] {
				continue
			}
			owners := q.Owners
			report.Registries = append(report.Registries, RegistryReport{
				ID:            "onchain:" + string(q.ChainSelector),
				Label:         fmt.Sprintf("chain %s (%s)", q.ChainSelector, abbreviate(q.Address)),
				Type:          string(registrytype.OnChain),
				ChainSelector: string(q.ChainSelector),
				Address:       q.Address,
				Owners:        &owners,
			})
		}
	}

	for _, r := range report.Registries {
		if r.Type == string(registrytype.OnChain) {
			report.hasLinkedOwners = true
			report.LinkedOwners = quotas.LinkedOwners
			break
		}
	}
	return report
}

func matchOnChain(regs []registryquotaclient.OnChainRegistry, chainSelector, address string) int {
	for i, q := range regs {
		if !strings.EqualFold(q.Address, address) {
			continue
		}
		if chainSelector != "" && string(q.ChainSelector) != chainSelector {
			continue
		}
		return i
	}
	return -1
}

func findRegistry(registries []*tenantctx.Registry, id string) *tenantctx.Registry {
	for _, r := range registries {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func registryIDs(registries []*tenantctx.Registry) []string {
	ids := make([]string, 0, len(registries))
	for _, r := range registries {
		ids = append(ids, r.ID)
	}
	return ids
}

func printReport(report Report) {
	ui.Line()
	ui.Bold("Registry quotas for your organization")
	ui.Line()

	if len(report.Registries) == 0 {
		ui.Warning("No registry quotas found for this environment")
		return
	}

	ownerStatus := linkStatusByAddress(report.LinkedOwners)
	staleContext := false
	for _, r := range report.Registries {
		ui.Bold(r.Label)
		ui.Dim(fmt.Sprintf("ID:   %s", r.ID))
		ui.Dim(fmt.Sprintf("Type: %s", r.Type))
		if r.Address != "" {
			ui.Dim(fmt.Sprintf("Addr: %s", r.Address))
		}
		if !r.InUserContext {
			staleContext = true
		}

		switch {
		case r.Workflows != nil:
			printOffChain(*r.Workflows)
		case r.Owners != nil:
			printOnChain(*r.Owners, ownerStatus)
		}
		ui.Line()
	}

	if report.hasLinkedOwners {
		printLinkedOwners(report.LinkedOwners)
		ui.Line()
	}

	if staleContext {
		ui.Dim("Some registries are missing from your saved user context. Run `cre login` to refresh it.")
		ui.Line()
	}
	for _, w := range report.Warnings {
		ui.Warning("Some quotas could not be read: " + w)
	}
}

func printOffChain(workflows []registryquotaclient.DonFamilyQuota) {
	if workflows == nil {
		ui.Warning("Workflow quota unavailable")
		return
	}
	ui.Print("Workflows by DON family:")
	if len(workflows) == 0 {
		ui.Dim("  No DON families available to your organization")
		return
	}
	printFamilies(workflows, "  ")
	if len(workflows) > 1 {
		ui.Dim("  Workflows are counted per organization, so every DON family shows the same total.")
	}
}

func printOnChain(owners []registryquotaclient.OwnerQuota, status map[string]string) {
	if owners == nil {
		ui.Warning("On-chain workflow quota unavailable")
		return
	}
	ui.Print("Workflows by owner and DON family:")
	if len(owners) == 0 {
		ui.Dim("  No linked owners. Link one with `cre account link-key`.")
		return
	}
	for _, o := range owners {
		label := o.Owner
		if s, ok := status[strings.ToLower(o.Owner)]; ok {
			label = fmt.Sprintf("%s (%s)", o.Owner, s)
		}
		ui.Print("  " + label)
		if len(o.Workflows) == 0 {
			ui.Dim("    No DON families available to this owner")
			continue
		}
		printFamilies(o.Workflows, "    ")
	}
}

func printFamilies(workflows []registryquotaclient.DonFamilyQuota, indent string) {
	sorted := append([]registryquotaclient.DonFamilyQuota(nil), workflows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DonFamily < sorted[j].DonFamily })

	width := 0
	for _, w := range sorted {
		width = max(width, len(w.DonFamily))
	}
	for _, w := range sorted {
		ui.Print(fmt.Sprintf("%s%-*s  %s", indent, width, w.DonFamily, formatUsage(w.Active, w.Limit)))
	}
}

func formatUsage(active, limit int) string {
	switch {
	case limit == 0:
		return fmt.Sprintf("disabled (%d active)", active)
	case active > limit:
		return fmt.Sprintf("%d / %d active (over limit)", active, limit)
	default:
		return fmt.Sprintf("%d / %d active, %d remaining", active, limit, limit-active)
	}
}

func printLinkedOwners(lo *registryquotaclient.LinkedOwners) {
	ui.Bold("Linked owners (all on-chain registries)")
	if lo == nil {
		ui.Warning("Linked-owner quota unavailable")
		return
	}
	used := len(lo.Linked) + len(lo.Pending)
	summary := fmt.Sprintf("%d / %d used (%d linked, %d pending)", used, lo.Limit, len(lo.Linked), len(lo.Pending))
	if used <= lo.Limit {
		summary += fmt.Sprintf(", %d remaining", lo.Limit-used)
	} else {
		summary += ", over limit"
	}
	ui.Print(summary)
	for _, o := range lo.Linked {
		ui.Dim(fmt.Sprintf("  %s  linked", o.Address))
	}
	for _, o := range lo.Pending {
		ui.Dim(fmt.Sprintf("  %s  pending", o.Address))
	}
	if len(lo.Pending) > 0 {
		ui.Dim("Pending link requests count against the limit until they expire.")
	}
}

func linkStatusByAddress(lo *registryquotaclient.LinkedOwners) map[string]string {
	status := map[string]string{}
	if lo == nil {
		return status
	}
	for _, o := range lo.Linked {
		status[strings.ToLower(o.Address)] = "linked"
	}
	for _, o := range lo.Pending {
		status[strings.ToLower(o.Address)] = "pending"
	}
	return status
}

func abbreviate(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}
