package get_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"

	cmdget "github.com/smartcontractkit/cre-cli/cmd/workflow/get"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func strPtr(s string) *string { return &s }

// workflowServer starts an httptest.Server that responds to ListWorkflows
// with the provided pages (each call advances through pages) and records the
// raw request bodies so tests can assert the GQL variables that were sent.
type workflowServer struct {
	*httptest.Server
	requests []string
}

func newWorkflowServer(t *testing.T, pages [][]map[string]string, totalCount int) *workflowServer {
	t.Helper()
	ws := &workflowServer{}
	var call atomic.Int32
	ws.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ws.requests = append(ws.requests, string(body))
		idx := int(call.Add(1)) - 1
		w.Header().Set("Content-Type", "application/json")
		var data []map[string]string
		if idx < len(pages) {
			data = pages[idx]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"workflows": map[string]any{
					"count": totalCount,
					"data":  data,
				},
			},
		})
	}))
	return ws
}

// buildSettings returns a minimal *settings.Settings populated with the
// workflow-name and deployment-registry under the "staging" target.
func buildSettings(workflowName, deploymentRegistry string) *settings.Settings {
	s := &settings.Settings{
		User: settings.UserSettings{TargetName: "staging"},
	}
	s.Workflow.UserWorkflowSettings.WorkflowName = workflowName
	s.Workflow.UserWorkflowSettings.DeploymentRegistry = deploymentRegistry
	return s
}

func newHandlerWithServer(t *testing.T, rtCtx *runtime.Context, srv *workflowServer) *cmdget.Handler {
	t.Helper()
	logger := zerolog.Nop()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	envSet := &environments.EnvironmentSet{GraphQLURL: srv.URL}
	if rtCtx.Credentials == nil {
		rtCtx.Credentials = creds
	}
	if rtCtx.EnvironmentSet == nil {
		rtCtx.EnvironmentSet = envSet
	}
	gql := graphqlclient.New(creds, envSet, &logger)
	wdc := workflowdataclient.New(gql, &logger)
	return cmdget.NewHandlerWithClient(rtCtx, wdc)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestExecute_NoTenantContext(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		Settings:       buildSettings("alpha", "private"),
	}

	h := cmdget.NewHandlerWithClient(rtCtx, nil)
	err := h.Execute(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "user context not available") {
		t.Fatalf("expected tenant-context error, got %v", err)
	}
}

func TestExecute_NoCredentials(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{Registries: []*tenantctx.Registry{{ID: "private"}}},
		Settings:       buildSettings("alpha", "private"),
	}

	h := cmdget.NewHandlerWithClient(rtCtx, nil)
	err := h.Execute(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "credentials not available") {
		t.Fatalf("expected credentials error, got %v", err)
	}
}

func TestExecute_MissingWorkflowName(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{Registries: []*tenantctx.Registry{{ID: "private"}}},
		Settings:       buildSettings("", "private"),
	}

	h := cmdget.NewHandlerWithClient(rtCtx, nil)
	err := h.Execute(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "workflow-name is not set") {
		t.Fatalf("expected missing workflow-name error, got %v", err)
	}
}

func TestExecute_UnknownDeploymentRegistry(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings: buildSettings("alpha", "does-not-exist"),
	}

	h := cmdget.NewHandlerWithClient(rtCtx, nil)
	err := h.Execute(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "not found in user context") {
		t.Fatalf("expected unknown registry error, got %v", err)
	}
}

func TestExecute_FiltersByDeploymentRegistry(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:testnet",
					ChainSelector: strPtr("12345678901234567890"),
					Address:       strPtr("0xcafebabe00000000000000000000000000feed"),
				},
				{ID: "private", Type: "off-chain"},
			},
		},
		Settings: buildSettings("alpha", "private"),
	}

	// Server returns two rows with the same name, on two different registries.
	page := []map[string]string{
		{
			"name":           "alpha",
			"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
			"ownerAddress":   "2020202020202020202020202020202020202020",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
		{
			"name":           "alpha",
			"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
			"ownerAddress":   "4040404040404040404040404040404040404040",
			"status":         "ACTIVE",
			"workflowSource": "contract:12345678901234567890:0xcafebabe00000000000000000000000000feed",
		},
	}
	srv := newWorkflowServer(t, [][]map[string]string{page}, len(page))
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Only the row whose source resolves to the "private" deployment-registry
	// should be printed.
	if got := strings.Count(out, "1. alpha"); got != 1 {
		t.Errorf("expected exactly one workflow row, got %d:\n%s", got, out)
	}
	wantID := "1010101010101010101010101010101010101010101010101010101010101010"
	if !strings.Contains(out, wantID) {
		t.Errorf("expected private-registry workflow id in output:\n%s", out)
	}
	if strings.Contains(out, "3030303030303030303030303030303030303030303030303030303030303030") {
		t.Errorf("on-chain row should have been filtered out:\n%s", out)
	}

	// The GQL call should have forwarded the workflow name in the search arg.
	if len(srv.requests) == 0 || !strings.Contains(srv.requests[0], `"search":"alpha"`) {
		t.Errorf("expected search variable to be set to workflow name; requests=%v", srv.requests)
	}
}

func TestExecute_AllRegistriesSkipsFilter(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:testnet",
					ChainSelector: strPtr("12345678901234567890"),
					Address:       strPtr("0xcafebabe00000000000000000000000000feed"),
				},
				{ID: "private", Type: "off-chain"},
			},
		},
		Settings: buildSettings("alpha", "private"),
	}

	page := []map[string]string{
		{
			"name":           "alpha",
			"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
			"ownerAddress":   "2020202020202020202020202020202020202020",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
		{
			"name":           "alpha",
			"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
			"ownerAddress":   "4040404040404040404040404040404040404040",
			"status":         "ACTIVE",
			"workflowSource": "contract:12345678901234567890:0xcafebabe00000000000000000000000000feed",
		},
	}
	srv := newWorkflowServer(t, [][]map[string]string{page}, len(page))
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if got := strings.Count(out, "alpha"); got < 2 {
		t.Errorf("expected both rows when --all-registries is set:\n%s", out)
	}
	if !strings.Contains(out, "0xcafebabe00000000000000000000000000feed") {
		t.Errorf("expected on-chain row to appear with --all-registries:\n%s", out)
	}
}

func TestExecute_ExactNameMatchNarrowsSearch(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings: buildSettings("alpha", "private"),
	}

	// The platform search matches substrings, so the server returns both
	// "alpha" and "alpha-staging" — only the exact match should be printed.
	page := []map[string]string{
		{
			"name":           "alpha",
			"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
			"ownerAddress":   "2020202020202020202020202020202020202020",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
		{
			"name":           "alpha-staging",
			"workflowId":     "5050505050505050505050505050505050505050505050505050505050505050",
			"ownerAddress":   "6060606060606060606060606060606060606060",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
	}
	srv := newWorkflowServer(t, [][]map[string]string{page}, len(page))
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "1. alpha") {
		t.Errorf("expected exact-match row in output:\n%s", out)
	}
	if strings.Contains(out, "alpha-staging") {
		t.Errorf("did not expect substring-only match alpha-staging:\n%s", out)
	}
}

func TestExecute_NoMatchPrintsEmptyState(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings: buildSettings("alpha", "private"),
	}

	srv := newWorkflowServer(t, [][]map[string]string{{}}, 0)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	var errOut string
	captureStdout(t, func() {
		errOut = captureStderr(t, func() {
			if err := h.Execute(context.Background(), false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	if !strings.Contains(errOut, "No workflows found") {
		t.Errorf("expected empty-state warning on stderr; got:\n%s", errOut)
	}
}

func TestNew_RequiresOneArg(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{},
		TenantContext:  &tenantctx.EnvironmentContext{},
		Settings:       buildSettings("alpha", "private"),
	}

	cmd := cmdget.New(rtCtx)
	cmd.SetArgs([]string{}) // no args
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no workflow folder path is provided")
	}
}
