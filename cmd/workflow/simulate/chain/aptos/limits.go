package aptos

import (
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

func ExtractLimits(w *cresettings.Workflows) chain.Limits {
	return chain.Limits{
		ReportSize: int(w.ChainWrite.ReportSizeLimit.DefaultValue),
		GasLimit:   w.ChainWrite.Aptos.GasLimit.Default.DefaultValue,
	}
}
