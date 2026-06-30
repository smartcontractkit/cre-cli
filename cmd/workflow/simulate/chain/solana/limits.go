package solana

import (
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

func ExtractLimits(w *cresettings.Workflows) chain.Limits {
	return chain.Limits{
		ReportSize: int(w.ChainWrite.Solana.ReportSizeLimit.DefaultValue),
		// Solana compute-unit limit is Setting[uint32]; widen to chain.Limits.GasLimit (uint64).
		GasLimit: uint64(w.ChainWrite.Solana.GasLimit.Default.DefaultValue),
	}
}
