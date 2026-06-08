package registrytype

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestFromGQL(t *testing.T) {
	log := testutil.NewTestLogger()
	tests := []struct {
		input string
		want  Type
	}{
		{GQLOnChain, OnChain},
		{GQLOffChain, OffChain},
		{"FUTURE_TYPE", Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, FromGQL(tt.input, log))
		})
	}
}

func TestParse(t *testing.T) {
	valid := []struct {
		input string
		want  Type
	}{
		{"on-chain", OnChain},
		{"off-chain", OffChain},
		{"unknown", Unknown},
	}
	for _, tt := range valid {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	invalid := []string{"ON-CHAIN", "OFF-CHAIN", "on_chain", "off_chain", "ON_CHAIN", "OFF_CHAIN", "on-chian", ""}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unrecognised registry type")
		})
	}
}
