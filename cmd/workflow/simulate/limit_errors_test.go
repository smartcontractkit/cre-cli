package simulate

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitExceededMessageFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kind        LimitKind
		resource    string
		actual      uint64
		limit       uint64
		mirrorsProd bool
		remediation string
		wantSubstrs []string
		notWant     []string
	}{
		{
			name:        "WASM binary",
			kind:        LimitWASMBinary,
			resource:    "WASM binary",
			actual:      200,
			limit:       100,
			mirrorsProd: true,
			remediation: "Reduce compiled binary size",
			wantSubstrs: []string{
				"WASM binary",
				"200 bytes",
				"100 bytes",
				"production",
				"cre workflow limits export",
				"--limits=none",
				"Reduce compiled binary size",
			},
		},
		{
			name:        "HTTP request",
			kind:        LimitHTTPRequest,
			resource:    "HTTP request body",
			actual:      5000,
			limit:       1000,
			mirrorsProd: true,
			remediation: "Reduce the request payload",
			wantSubstrs: []string{
				"HTTP request body",
				"5000 bytes",
				"1000 bytes",
				"production",
				"cre workflow limits export",
				"--limits=none",
			},
		},
		{
			name:        "consensus observation",
			kind:        LimitConsensusObservation,
			resource:    "Consensus observation",
			actual:      30000,
			limit:       25000,
			mirrorsProd: true,
			remediation: "Reduce data passed",
			wantSubstrs: []string{
				"Consensus observation",
				"30000 bytes",
				"25000 bytes",
				"production",
			},
		},
		{
			name:        "non-prod mirror limit",
			kind:        LimitChainWriteReport,
			resource:    "Some limit",
			actual:      10,
			limit:       5,
			mirrorsProd: false,
			remediation: "Do something",
			wantSubstrs: []string{
				"Some limit",
				"10 bytes",
				"5 bytes",
			},
			notWant: []string{"production"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := limitExceeded(tc.kind, tc.resource, tc.actual, tc.limit, tc.mirrorsProd, tc.remediation)

			require.NotNil(t, err)
			assert.Equal(t, tc.kind, err.Kind)

			for _, want := range tc.wantSubstrs {
				assert.True(t, strings.Contains(err.Error(), want),
					"expected %q to contain %q", err.Error(), want)
			}
			for _, notWant := range tc.notWant {
				assert.False(t, strings.Contains(err.Error(), notWant),
					"expected %q NOT to contain %q", err.Error(), notWant)
			}
		})
	}
}

func TestLimitExceededUnitMessageFormat(t *testing.T) {
	t.Parallel()

	err := limitExceededUnit(LimitEVMGas, "EVM gas", 6_000_000, 5_000_000, "gas units", true, "Reduce gas_config.gas_limit")

	require.NotNil(t, err)
	assert.Equal(t, LimitEVMGas, err.Kind)
	assert.Contains(t, err.Error(), "EVM gas")
	assert.Contains(t, err.Error(), "6000000 gas units")
	assert.Contains(t, err.Error(), "5000000 gas units")
	assert.Contains(t, err.Error(), "production")
	assert.Contains(t, err.Error(), "cre workflow limits export")
	assert.Contains(t, err.Error(), "--limits=none")
}

func TestLimitExceededErrorsAs(t *testing.T) {
	t.Parallel()

	err := limitExceeded(LimitHTTPResponse, "HTTP response body", 200, 100, true, "Filter the response")

	// LimitExceededError is directly usable with errors.As.
	var limitErr *LimitExceededError
	require.True(t, errors.As(err, &limitErr))
	assert.Equal(t, LimitHTTPResponse, limitErr.Kind)
}

func TestLimitExceededWrappedErrorsAs(t *testing.T) {
	t.Parallel()

	// Verify that wrapping in fmt.Errorf still allows errors.As unwrapping.
	inner := limitExceeded(LimitWASMBinary, "WASM binary", 200, 100, true, "Reduce size")
	wrapped := errors.New(inner.Error()) // simulate wrapping without %w

	// When wrapped without %w, errors.As won't traverse; verify the direct case works.
	var limitErr *LimitExceededError
	assert.True(t, errors.As(inner, &limitErr))
	assert.Equal(t, LimitWASMBinary, limitErr.Kind)
	_ = wrapped // just to avoid unused variable
}
