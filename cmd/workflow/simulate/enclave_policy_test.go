package simulate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
)

func TestWarnIfRejectedByEnclave(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		req       *confidentialhttp.HTTPRequest
		wantWarns int
	}{
		{
			name:      "compliant request",
			req:       &confidentialhttp.HTTPRequest{Url: "https://api.example.com/v1", Method: "GET"},
			wantWarns: 0,
		},
		{
			name:      "https on port 443 is compliant",
			req:       &confidentialhttp.HTTPRequest{Url: "https://api.example.com:443/v1", Method: "POST"},
			wantWarns: 0,
		},
		{
			name:      "plain http warns on scheme",
			req:       &confidentialhttp.HTTPRequest{Url: "http://api.example.com/v1", Method: "GET"},
			wantWarns: 1,
		},
		{
			name:      "localhost mock server warns but is not blocked",
			req:       &confidentialhttp.HTTPRequest{Url: "http://localhost:8080/mock", Method: "GET"},
			wantWarns: 2, // scheme + port
		},
		{
			name:      "disallowed method warns",
			req:       &confidentialhttp.HTTPRequest{Url: "https://api.example.com/v1", Method: "TRACE"},
			wantWarns: 1,
		},
		{
			name:      "all three violations",
			req:       &confidentialhttp.HTTPRequest{Url: "http://api.example.com:8080/v1", Method: "CONNECT"},
			wantWarns: 3,
		},
		{
			name:      "nil request is a no-op",
			req:       nil,
			wantWarns: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lggr, logs := logger.TestObserved(t, zapcore.WarnLevel)
			warnIfRejectedByEnclave(lggr, tt.req)
			assert.Equal(t, tt.wantWarns, logs.Len(), "unexpected warning count")
		})
	}
}

func TestTeeRequirementSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tee  *sdkpb.Tee
		want string
	}{
		{
			name: "nil",
			tee:  nil,
			want: "TEE",
		},
		{
			name: "typed with regions",
			tee: &sdkpb.Tee{Item: &sdkpb.Tee_TeeTypesAndRegions{TeeTypesAndRegions: &sdkpb.TeeTypesAndRegions{
				TeeTypeAndRegions: []*sdkpb.TeeTypeAndRegions{{
					Type:    sdkpb.TeeType_TEE_TYPE_AWS_NITRO,
					Regions: []string{"us-west-2"},
				}},
			}}},
			want: "AWS_NITRO (us-west-2)",
		},
		{
			name: "any region with regions",
			tee: &sdkpb.Tee{Item: &sdkpb.Tee_AnyRegions{AnyRegions: &sdkpb.Regions{
				Regions: []string{"us-west-2", "eu-west-1"},
			}}},
			want: "TEE (us-west-2, eu-west-1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, teeRequirementSummary(tt.tee))
		})
	}
}
