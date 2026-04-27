package aptos

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
)

func TestExtractLimitsFromDefault(t *testing.T) {
	t.Parallel()
	w := cresettings.Default.PerWorkflow
	lim := ExtractLimits(&w)
	assert.Equal(t, 5_000, lim.ReportSize)
	assert.Equal(t, uint64(2_000_000), lim.GasLimit)
}
