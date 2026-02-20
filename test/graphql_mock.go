package test

import (
	"net/http/httptest"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

// NewGraphQLMockServerGetOrganization starts a mock GraphQL server that responds to
// getOrganization and sets EnvVarGraphQLURL. Caller must defer srv.Close().
func NewGraphQLMockServerGetOrganization(t *testing.T) *httptest.Server {
	return testutil.NewGraphQLMockServerGetOrganization(t)
}
