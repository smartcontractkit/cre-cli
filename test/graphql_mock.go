package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/telemetry"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

// NewGraphQLMockServerGetOrganization starts a mock GraphQL server that responds to
// getCreOrganizationInfo and sets EnvVarGraphQLURL. Caller must defer srv.Close().
func NewGraphQLMockServerGetOrganization(t *testing.T) *httptest.Server {
	return testutil.NewGraphQLMockServerGetOrganization(t)
}

// RecordedTelemetry gives thread-safe access to the reportUserEvent mutations
// received by NewGraphQLMockServerRecordingTelemetry. The CLI sends telemetry
// from a goroutine killed by os.Exit, so callers must poll Events() briefly
// after the CLI process exits rather than reading it immediately.
type RecordedTelemetry struct {
	mu     sync.Mutex
	events []telemetry.UserEventInput
}

// Events returns a snapshot of the reportUserEvent mutations received so far.
func (r *RecordedTelemetry) Events() []telemetry.UserEventInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]telemetry.UserEventInput(nil), r.events...)
}

func (r *RecordedTelemetry) record(e telemetry.UserEventInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

// NewGraphQLMockServerRecordingTelemetry starts an httptest.Server that answers
// getCreOrganizationInfo/getTenantConfig like NewGraphQLMockServerGetOrganization,
// and additionally records every reportUserEvent mutation it receives so tests
// can assert telemetry was actually sent over the wire. It sets EnvVarGraphQLURL.
// Caller must defer srv.Close().
func NewGraphQLMockServerRecordingTelemetry(t *testing.T) (*httptest.Server, *RecordedTelemetry) {
	t.Helper()
	testutil.IsolateCLIHome(t)

	rec := &RecordedTelemetry{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost {
			var req struct {
				Query     string `json:"query"`
				Variables struct {
					Event telemetry.UserEventInput `json:"event"`
				} `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(req.Query, "reportUserEvent") {
				rec.record(req.Variables.Event)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"reportUserEvent": map[string]any{"success": true, "message": ""},
					},
				})
				return
			}
			if strings.Contains(req.Query, "getCreOrganizationInfo") {
				_ = json.NewEncoder(w).Encode(testutil.MockGetCreOrganizationInfoGraphQLPayload())
				return
			}
			if testutil.QueryIsGetTenantConfig(req.Query) {
				_ = json.NewEncoder(w).Encode(testutil.MockGetTenantConfigGraphQLPayload())
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
		}
	}))
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")
	return srv, rec
}
