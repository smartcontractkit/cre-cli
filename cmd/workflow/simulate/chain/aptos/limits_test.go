package aptos

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
)

func TestExtractLimitsFromDefault(t *testing.T) {
	t.Parallel()
	w := cresettings.Default.PerWorkflow
	lim := ExtractLimits(&w)
	assert.Equal(t, 5_000, lim.ReportSize)
	assert.Equal(t, uint64(2_000_000), lim.GasLimit)
}

func TestExtractLimitsAfterJSONOverride(t *testing.T) {
	t.Parallel()
	w := cresettings.Default.PerWorkflow
	require.NoError(t, json.Unmarshal([]byte(`{
		"ChainWrite": {"Aptos": {"GasLimit": {"Default": "456"}}}
	}`), &w))
	assert.Equal(t, uint64(456), ExtractLimits(&w).GasLimit)
}
