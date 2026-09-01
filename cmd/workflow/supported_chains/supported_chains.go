package supported_chains

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain/evm"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const outputFormatJSON = "json"

// ChainForwarderRow is one tenant chain with mock forwarder address for JSON output.
type ChainForwarderRow struct {
	ChainName     string `json:"chainName"`
	ChainSelector uint64 `json:"chainSelector"`
	Address       string `json:"address"`
	MockAddress   string `json:"mockAddress"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "supported-chains",
		Short: "List chains, forwarder addresses and mock forwarder addresses for your tenant",
		Long: "Lists chain selectors, Keystone forwarder contract addresses and mock forwarder addresses returned by the platform " +
			"for the current tenant (from cre login / CRE_API_KEY). Chains are those enabled for your tenant.",
		Example: "cre workflow supported-chains\n" +
			"  cre workflow supported-chains --output json",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if outputFormat != "" && outputFormat != outputFormatJSON {
				return fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
			}

			if runtimeContext == nil || runtimeContext.TenantContext == nil {
				return fmt.Errorf("user context not available — run `cre login` and retry")
			}

			fwd := runtimeContext.TenantContext.Forwarders
			if len(fwd) == 0 {
				ui.Print("No forwarders returned for this tenant.")
				ui.Dim("If you recently upgraded the CLI, run cre login again (or set CRE_API_KEY) to refresh context.")
				return nil
			}

			rows := make([]ChainForwarderRow, 0, len(fwd))
			for _, f := range fwd {
				name := "-"
				if n, err := settings.GetChainNameByChainSelector(f.ChainSelector); err == nil {
					name = n
				}
				var mockForwarderAddress string
				for _, chain := range evm.SupportedChains {
					if chain.Selector == f.ChainSelector {
						mockForwarderAddress = chain.Forwarder
					}
				}
				rows = append(rows, ChainForwarderRow{
					ChainName:     name,
					ChainSelector: f.ChainSelector,
					Address:       f.Address,
					MockAddress:   mockForwarderAddress,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].ChainSelector != rows[j].ChainSelector {
					return rows[i].ChainSelector < rows[j].ChainSelector
				}
				return rows[i].Address < rows[j].Address
			})

			if outputFormat == outputFormatJSON {
				out, err := json.MarshalIndent(rows, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to serialize chains as JSON: %w", err)
				}
				ui.Print(string(out))
				return nil
			}

			ui.Print("Tenant-scoped chains, forwarders and mock forwarders:")
			ui.Line()

			var buf strings.Builder
			w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "CHAIN\tSELECTOR\tFORWARDER ADDRESS\tMOCK FORWARDER ADDRESS"); err != nil {
				return err
			}
			for _, r := range rows {
				if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", r.ChainName, r.ChainSelector, r.Address, r.MockAddress); err != nil {
					return err
				}
			}
			if err := w.Flush(); err != nil {
				return err
			}
			ui.Print(strings.TrimSuffix(buf.String(), "\n"))
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	return cmd
}
