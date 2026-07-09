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
	"time"

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
	err := h.Execute(context.Background(), cmdget.Inputs{})
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
	err := h.Execute(context.Background(), cmdget.Inputs{})
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
	err := h.Execute(context.Background(), cmdget.Inputs{})
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
	err := h.Execute(context.Background(), cmdget.Inputs{})
	if err == nil || !strings.Contains(err.Error(), "not found in user context") {
		t.Fatalf("expected unknown registry error, got %v", err)
	}
}

func TestExecute_FiltersByDeploymentRegistry(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 2,
					"data": []any{
						map[string]any{
							"uuid":           "wf-private",
							"name":           "alpha",
							"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
							"ownerAddress":   "2020202020202020202020202020202020202020",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
						map[string]any{
							"uuid":           "wf-onchain",
							"name":           "alpha",
							"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
							"ownerAddress":   "4040404040404040404040404040404040404040",
							"status":         "ACTIVE",
							"workflowSource": "contract:12345678901234567890:0xcafebabe00000000000000000000000000feed",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-private",
						"name":           "alpha",
						"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
						"ownerAddress":   "2020202020202020202020202020202020202020",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executionCount": 0,
						"executionCountByStatus": map[string]any{
							"success": 0,
							"failure": 0,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlRespond(w, map[string]any{"workflowDeployments": map[string]any{"data": []any{}}})
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{"workflowExecutions": map[string]any{"count": 0, "data": []any{}}})
		default:
			gqlRespond(w, map[string]any{})
		}
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
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
		Settings: buildSettingsWithOwner("alpha", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdget.Inputs{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	wantID := "1010101010101010101010101010101010101010101010101010101010101010"
	if !strings.Contains(out, wantID) {
		t.Errorf("expected private-registry workflow id in output:\n%s", out)
	}
	if strings.Contains(out, "3030303030303030303030303030303030303030303030303030303030303030") {
		t.Errorf("on-chain row should have been filtered out:\n%s", out)
	}
}

func TestExecute_AllRegistriesWithMultipleActiveErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflows": map[string]any{
				"count": 2,
				"data": []any{
					map[string]any{
						"uuid":           "wf-private",
						"name":           "alpha",
						"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
					},
					map[string]any{
						"uuid":           "wf-onchain",
						"name":           "alpha",
						"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "contract:12345678901234567890:0xcafebabe00000000000000000000000000feed",
					},
				},
			},
		})
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
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
		Settings: buildSettingsWithOwner("alpha", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	err := h.Execute(context.Background(), cmdget.Inputs{AllRegistries: true})
	if err == nil || !strings.Contains(err.Error(), "multiple ACTIVE workflows") {
		t.Fatalf("expected multiple ACTIVE error, got %v", err)
	}
}

func TestExecute_ExactNameMatchNarrowsSearch(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 2,
					"data": []any{
						map[string]any{
							"uuid":           "wf-alpha",
							"name":           "alpha",
							"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
						map[string]any{
							"uuid":           "wf-alpha-staging",
							"name":           "alpha-staging",
							"workflowId":     "5050505050505050505050505050505050505050505050505050505050505050",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-alpha",
						"name":           "alpha",
						"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executionCount": 0,
						"executionCountByStatus": map[string]any{
							"success": 0,
							"failure": 0,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlRespond(w, map[string]any{"workflowDeployments": map[string]any{"data": []any{}}})
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{"workflowExecutions": map[string]any{"count": 0, "data": []any{}}})
		default:
			gqlRespond(w, map[string]any{})
		}
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings: buildSettingsWithOwner("alpha", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdget.Inputs{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Workflow: alpha") {
		t.Errorf("expected exact-match row in output:\n%s", out)
	}
	if strings.Contains(out, "alpha-staging") {
		t.Errorf("did not expect substring-only match alpha-staging:\n%s", out)
	}
}

func TestExecute_NoMatchReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflows": map[string]any{"data": []any{}, "count": 0},
		})
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings: buildSettingsWithOwner("alpha", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	err := h.Execute(context.Background(), cmdget.Inputs{})
	if err == nil || !strings.Contains(err.Error(), `no workflow found with name "alpha"`) {
		t.Fatalf("expected not-found error, got %v", err)
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

func buildSettingsWithOwner(workflowName, owner string) *settings.Settings {
	s := buildSettings(workflowName, "private")
	s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress = owner
	return s
}

func gqlRespond(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

func gqlError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{{"message": msg}},
	})
}

func TestExecute_MissingSettings(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{},
	}

	h := cmdget.NewHandlerWithClient(rtCtx, nil)
	err := h.Execute(context.Background(), cmdget.Inputs{})
	if err == nil || !strings.Contains(err.Error(), "workflow settings not loaded") {
		t.Fatalf("expected missing settings error, got %v", err)
	}
}

func TestExecute_WorkflowNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflows": map[string]any{"data": []any{}, "count": 0},
		})
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings:       buildSettingsWithOwner("missing-workflow", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	err := h.Execute(context.Background(), cmdget.Inputs{})
	if err == nil || !strings.Contains(err.Error(), `no workflow found with name "missing-workflow"`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestExecute_JSONOutput(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	executed := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	deployed := time.Date(2026, 1, 10, 11, 55, 0, 0, time.UTC)
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)
	txHash := "0xabc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":           "wf-uuid-1",
							"name":           "my-workflow",
							"workflowId":     "abc123onchain",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-uuid-1",
						"name":           "my-workflow",
						"workflowId":     "abc123onchain",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executedAt":     executed.Format(time.RFC3339),
						"executionCount": 42,
						"executionCountByStatus": map[string]any{
							"success": 40,
							"failure": 2,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlRespond(w, map[string]any{
				"workflowDeployments": map[string]any{
					"data": []any{
						map[string]any{
							"uuid":       "dep-uuid-1",
							"workflowID": "abc123onchain",
							"status":     "SUCCESS",
							"deployedAt": deployed.Format(time.RFC3339),
							"txHash":     txHash,
						},
					},
				},
			})
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{
				"workflowExecutions": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":         "exec-uuid-1",
							"workflowUUID": "wf-uuid-1",
							"workflowName": "my-workflow",
							"status":       "SUCCESS",
							"startedAt":    started.Format(time.RFC3339),
							"finishedAt":   finished.Format(time.RFC3339),
							"errors":       []any{},
						},
					},
				},
			})
		default:
			t.Errorf("unexpected query: %s", query)
			gqlRespond(w, map[string]any{})
		}
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings:       buildSettingsWithOwner("my-workflow", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdget.Inputs{OutputFormat: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}

	workflow, ok := result["workflow"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow object in JSON: %s", out)
	}
	if workflow["name"] != "my-workflow" {
		t.Errorf("expected name my-workflow, got %v", workflow["name"])
	}
	if workflow["ownerAddress"] != "0xowner" {
		t.Errorf("expected ownerAddress 0xowner, got %v", workflow["ownerAddress"])
	}
	if _, ok := workflow["executionCount"]; ok {
		t.Errorf("executionCount should not be in JSON output")
	}

	deployment, ok := result["deployment"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment object in JSON: %s", out)
	}
	if deployment["txHash"] != txHash {
		t.Errorf("expected txHash %s, got %v", txHash, deployment["txHash"])
	}

	lastExec, ok := result["lastExecution"].(map[string]any)
	if !ok {
		t.Fatalf("expected lastExecution object in JSON: %s", out)
	}
	if lastExec["uuid"] != "exec-uuid-1" {
		t.Errorf("expected exec uuid, got %v", lastExec["uuid"])
	}
}

func TestExecute_ContinuesWhenDeploymentUnavailable(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":           "wf-uuid-1",
							"name":           "my-workflow",
							"workflowId":     "abc123onchain",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-uuid-1",
						"name":           "my-workflow",
						"workflowId":     "abc123onchain",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executionCount": 0,
						"executionCountByStatus": map[string]any{
							"success": 0,
							"failure": 0,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlError(w, "deployment service unavailable")
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{
				"workflowExecutions": map[string]any{"count": 0, "data": []any{}},
			})
		default:
			gqlRespond(w, map[string]any{})
		}
	}))
	defer srv.Close()

	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: srv.URL},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Type: "off-chain"}},
		},
		Settings:       buildSettingsWithOwner("my-workflow", "0xowner"),
	}
	h := newHandlerWithServer(t, rtCtx, &workflowServer{Server: srv})

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdget.Inputs{OutputFormat: "json"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["workflow"]; !ok {
		t.Fatal("expected workflow in output")
	}
	if _, ok := result["deployment"]; ok {
		t.Fatal("deployment should be omitted when unavailable")
	}
}

func TestNew_InvalidOutputFormat(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{},
		TenantContext:  &tenantctx.EnvironmentContext{},
		Settings:       buildSettingsWithOwner("my-workflow", "0xowner"),
	}

	cmd := cmdget.New(rtCtx)
	cmd.SetArgs([]string{"./my-workflow", "--output", "csv"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "csv") {
		t.Fatalf("expected invalid output format error, got %v", err)
	}
}
