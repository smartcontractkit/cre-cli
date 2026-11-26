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
addresses are linked or pending against the linked-owner limit.

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

	// A partial result is rendered with unreadable quotas shown as unavailable; any
	// other error means there is nothing to show.
	var partial *registryquotaclient.PartialError
	if err != nil && !errors.As(err, &partial) {
		return err
	}

	failed := map[string]bool{}
	if partial != nil {
		for _, key := range registryquotaclient.FailedQuotaKeys(partial) {
			failed[key] = true
		}
	}

	report := buildReport(h.tenantContext.Registries, quotas, registryID, failed)
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
	LinkedOwners    *LinkedOwnersReport `json:"linkedOwners"`
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
	ID            string            `json:"id"`
	Label         string            `json:"label"`
	Type          string            `json:"type"`
	ChainSelector string            `json:"chainSelector,omitempty"`
	Address       string            `json:"address,omitempty"`
	Workflows     *[]DonFamilyQuota `json:"workflows,omitempty"`
	Owners        *[]OwnerQuota     `json:"owners,omitempty"`
	// InUserContext is false for a registry the API returned that the cached user
	// context does not know about yet.
	InUserContext bool `json:"-"`
}

// DonFamilyQuota is the usage and limit of one DON family.
type DonFamilyQuota struct {
	DonFamily string                      `json:"donFamily"`
	Active    *registryquotaclient.Amount `json:"active"`
	Limit     registryquotaclient.Amount  `json:"limit"`
}

// OwnerQuota is the usage and limit of one owner in one on-chain registry.
type OwnerQuota struct {
	Owner     string           `json:"owner"`
	Workflows []DonFamilyQuota `json:"workflows"`
}

// LinkedOwnersReport is the linked-owner quota of the organization, across all
// on-chain registries. Used is null when usage is not measured.
type LinkedOwnersReport struct {
	Used      *registryquotaclient.Amount `json:"used"`
	Limit     registryquotaclient.Amount  `json:"limit"`
	Remaining *registryquotaclient.Amount `json:"remaining"`
}

// onchainGroup is the quota of one on-chain registry, keyed by the quota dimensions
// the API returns. It may not be in the cached user context yet.
type onchainGroup struct {
	registry        string
	registryAddress string
	owners          map[string][]DonFamilyQuota
	ownerOrder      []string
}

func buildReport(registries []*tenantctx.Registry, quotas *registryquotaclient.Quotas, registryID string, failed map[string]bool) Report {
	report := Report{Registries: []RegistryReport{}}

	private := privateQuotas(quotas.RegistryLimits)
	onchain := onchainQuotas(quotas.RegistryLimits)
	usedPrivate := false
	usedOnChain := make([]bool, len(onchain))

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
			usedPrivate = true
			if failed[registryquotaclient.QuotaKeyPrivateWorkflowLimit] && len(private) == 0 {
				// A pointer to a nil slice renders as null: the quota could not be read.
				var unavailable []DonFamilyQuota
				entry.Workflows = &unavailable
			} else {
				workflows := private
				entry.Workflows = &workflows
			}
		case registrytype.OnChain:
			i := matchOnChain(onchain, reg)
			if i >= 0 {
				usedOnChain[i] = true
				owners := onchain[i].ownerList()
				if failed[registryquotaclient.QuotaKeyOnchainWorkflowLimit] && len(owners) == 0 {
					var unavailable []OwnerQuota
					entry.Owners = &unavailable
				} else {
					entry.Owners = &owners
				}
			} else if failed[registryquotaclient.QuotaKeyOnchainWorkflowLimit] {
				var unavailable []OwnerQuota
				entry.Owners = &unavailable
			} else {
				owners := []OwnerQuota{}
				entry.Owners = &owners
			}
		default:
			continue
		}

		if registryID == "" || registryID == reg.ID {
			report.Registries = append(report.Registries, entry)
		}
	}

	// Registries the cached user context does not know about yet.
	if registryID == "" {
		if !usedPrivate && len(private) > 0 {
			workflows := private
			report.Registries = append(report.Registries, RegistryReport{
				ID:        "private",
				Label:     "Private (Chainlink-hosted)",
				Type:      string(registrytype.OffChain),
				Workflows: &workflows,
			})
		}
		for i, g := range onchain {
			if usedOnChain[i] {
				continue
			}
			owners := g.ownerList()
			report.Registries = append(report.Registries, RegistryReport{
				ID:      onchainID(g),
				Label:   onchainLabel(g),
				Type:    string(registrytype.OnChain),
				Address: g.registryAddress,
				Owners:  &owners,
			})
		}
	}

	for _, r := range report.Registries {
		if r.Type == string(registrytype.OnChain) {
			report.hasLinkedOwners = true
			report.LinkedOwners = linkedOwners(quotas, failed)
			break
		}
	}
	return report
}

// privateQuotas returns the per-DON-family quota of the private registry. The API
// repeats the organization-wide usage on every family.
func privateQuotas(limits []registryquotaclient.ServiceUsageLimit) []DonFamilyQuota {
	workflows := []DonFamilyQuota{}
	for _, l := range limits {
		if l.QuotaKey != registryquotaclient.QuotaKeyPrivateWorkflowLimit {
			continue
		}
		workflows = append(workflows, donFamilyQuota(l))
	}
	return workflows
}

// onchainQuotas groups the per-owner quotas by on-chain registry. A registry may be
// absent from the cached user context, so it is identified by its address when set.
func onchainQuotas(limits []registryquotaclient.ServiceUsageLimit) []*onchainGroup {
	groups := []*onchainGroup{}
	index := map[string]*onchainGroup{}
	for _, l := range limits {
		if l.QuotaKey != registryquotaclient.QuotaKeyOnchainWorkflowLimit {
			continue
		}
		registry := l.Dimension(registryquotaclient.DimensionRegistry)
		address := l.Dimension(registryquotaclient.DimensionRegistryAddress)
		key := strings.ToLower(address)
		if key == "" {
			key = "registry:" + registry
		}
		g, ok := index[key]
		if !ok {
			g = &onchainGroup{registry: registry, registryAddress: address, owners: map[string][]DonFamilyQuota{}}
			index[key] = g
			groups = append(groups, g)
		}
		owner := l.Dimension(registryquotaclient.DimensionOwner)
		if _, ok := g.owners[owner]; !ok {
			g.ownerOrder = append(g.ownerOrder, owner)
		}
		g.owners[owner] = append(g.owners[owner], donFamilyQuota(l))
	}
	return groups
}

func donFamilyQuota(l registryquotaclient.ServiceUsageLimit) DonFamilyQuota {
	return DonFamilyQuota{
		DonFamily: l.Dimension(registryquotaclient.DimensionDonFamily),
		Active:    l.CurrentValue,
		Limit:     l.TotalLimit,
	}
}

func (g *onchainGroup) ownerList() []OwnerQuota {
	owners := make([]OwnerQuota, 0, len(g.ownerOrder))
	for _, owner := range g.ownerOrder {
		owners = append(owners, OwnerQuota{Owner: owner, Workflows: g.owners[owner]})
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].Owner < owners[j].Owner })
	return owners
}

func onchainID(g *onchainGroup) string {
	if g.registry != "" {
		return "onchain:" + g.registry
	}
	return "onchain:" + strings.ToLower(g.registryAddress)
}

func onchainLabel(g *onchainGroup) string {
	switch {
	case g.registry != "":
		return fmt.Sprintf("%s (%s)", g.registry, abbreviate(g.registryAddress))
	default:
		return abbreviate(g.registryAddress)
	}
}

func linkedOwners(quotas *registryquotaclient.Quotas, failed map[string]bool) *LinkedOwnersReport {
	if failed[registryquotaclient.QuotaKeyLinkedOwnersLimit] {
		return nil
	}
	for _, l := range quotas.OwnerLimits {
		if l.QuotaKey != registryquotaclient.QuotaKeyLinkedOwnersLimit {
			continue
		}
		report := &LinkedOwnersReport{Limit: l.TotalLimit}
		if l.CurrentValue != nil {
			used := *l.CurrentValue
			remaining := l.TotalLimit.Sub(used)
			report.Used = &used
			report.Remaining = &remaining
		}
		return report
	}
	return nil
}

func matchOnChain(groups []*onchainGroup, reg *tenantctx.Registry) int {
	chainName := strings.TrimPrefix(reg.ID, "onchain:")
	address := ""
	if reg.Address != nil {
		address = *reg.Address
	}
	for i, g := range groups {
		if g.registryAddress != "" && address != "" && strings.EqualFold(g.registryAddress, address) {
			return i
		}
	}
	for i, g := range groups {
		if g.registry != "" && chainName != "" && strings.EqualFold(g.registry, chainName) {
			return i
		}
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
			printOnChain(*r.Owners)
		case r.Type == string(registrytype.OffChain):
			ui.Warning("Workflow quota unavailable")
		default:
			ui.Warning("On-chain workflow quota unavailable")
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

func printOffChain(workflows []DonFamilyQuota) {
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

func printOnChain(owners []OwnerQuota) {
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
		ui.Print("  " + o.Owner)
		if len(o.Workflows) == 0 {
			ui.Dim("    No DON families available to this owner")
			continue
		}
		printFamilies(o.Workflows, "    ")
	}
}

func printFamilies(workflows []DonFamilyQuota, indent string) {
	sorted := append([]DonFamilyQuota(nil), workflows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DonFamily < sorted[j].DonFamily })

	width := 0
	for _, w := range sorted {
		width = max(width, len(w.DonFamily))
	}
	for _, w := range sorted {
		ui.Print(fmt.Sprintf("%s%-*s  %s", indent, width, w.DonFamily, formatUsage(w.Active, w.Limit)))
	}
}

func formatUsage(active *registryquotaclient.Amount, limit registryquotaclient.Amount) string {
	switch {
	case limit.IsZero():
		if active == nil {
			return "disabled"
		}
		return fmt.Sprintf("disabled (%s active)", active)
	case active == nil:
		return fmt.Sprintf("usage not measured, %s limit", limit)
	case active.Cmp(limit) > 0:
		return fmt.Sprintf("%s / %s active (over limit)", active, limit)
	default:
		return fmt.Sprintf("%s / %s active, %s remaining", active, limit, limit.Sub(*active))
	}
}

func printLinkedOwners(lo *LinkedOwnersReport) {
	ui.Bold("Linked owners (all on-chain registries)")
	if lo == nil {
		ui.Warning("Linked-owner quota unavailable")
		return
	}
	if lo.Used == nil {
		ui.Print(fmt.Sprintf("Usage not measured (limit %s)", lo.Limit))
		return
	}
	used := *lo.Used
	if used.Cmp(lo.Limit) > 0 {
		ui.Print(fmt.Sprintf("%s / %s used, over limit", used, lo.Limit))
	} else {
		ui.Print(fmt.Sprintf("%s / %s used, %s remaining", used, lo.Limit, lo.Limit.Sub(used)))
	}
	ui.Dim("Pending link requests count against the limit until they expire.")
}

func abbreviate(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}
