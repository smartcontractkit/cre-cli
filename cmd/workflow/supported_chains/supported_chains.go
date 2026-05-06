package supported_chains

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain/evm"
)

const outputFormatJSON = "json"

func New() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "supported-chains",
		Short: "List all supported chain names",
		Example: "cre workflow supported-chains\n" +
			"  cre workflow supported-chains --output json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFormat != "" && outputFormat != outputFormatJSON {
				return fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
			}

			names := evm.SupportedChainNames()

			if outputFormat == outputFormatJSON {
				out, err := json.MarshalIndent(names, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to serialize chain names as JSON: %w", err)
				}
				fmt.Println(string(out))
				return nil
			}

			fmt.Println("Supported chain names:")
			for _, name := range names {
				fmt.Printf("  %s\n", name)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	return cmd
}
