package list_test

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

	cmdlist "github.com/smartcontractkit/cre-cli/cmd/workflow/list"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func strPtr(s string) *string { return &s }

// newWorkflowServer starts an httptest.Server that responds to ListWorkflows
// with the provided pages of workflow data (each call advances through pages).
func newWorkflowServer(t *testing.T, pages [][]map[string]string, totalCount int) *httptest.Server {
	t.Helper()
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	return srv
}

// newHandlerWithServer builds a Handler wired to an httptest.Server.
func newHandlerWithServer(t *testing.T, rtCtx *runtime.Context, srv *httptest.Server) *cmdlist.Handler {
	t.Helper()
	logger := zerolog.Nop()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	envSet := &environments.EnvironmentSet{GraphQLURL: srv.URL}
	gql := graphqlclient.New(creds, envSet, &logger)
	wdc := workflowdataclient.New(gql, &logger)
	return cmdlist.NewHandlerWithClient(rtCtx, wdc)
}

// threeWorkflowPage returns the two-active-one-deleted page used across several tests.
func threeWorkflowPage() []map[string]string {
	return []map[string]string{
		{
			"name":           "alpha",
			"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
			"ownerAddress":   "2020202020202020202020202020202020202020",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
		{
			"name":           "beta",
			"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
			"ownerAddress":   "4040404040404040404040404040404040404040",
			"status":         "PAUSED",
			"workflowSource": "contract:999888777666555444333:0xabababababababababababababababababababab",
		},
		{
			"name":           "gone-deleted",
			"workflowId":     "5050505050505050505050505050505050505050505050505050505050505050",
			"ownerAddress":   "6060606060606060606060606060606060606060",
			"status":         "DELETED",
			"workflowSource": "private",
		},
	}
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

func TestNew_NoTenantContext(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  nil,
	}

	cmd := cmdlist.New(rtCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when TenantContext is nil")
	}
	if !strings.Contains(err.Error(), "user context not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_NoCredentials(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    nil,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{Registries: []*tenantctx.Registry{{ID: "private"}}},
	}

	cmd := cmdlist.New(rtCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when Credentials is nil")
	}
	if !strings.Contains(err.Error(), "credentials not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_UnknownRegistry(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Label: "Private"}},
		},
	}

	cmd := cmdlist.New(rtCtx)
	cmd.SetArgs([]string{"--registry", "nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown registry")
	}
	if !strings.Contains(err.Error(), "not found in user context") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_RejectsArgs(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{},
		TenantContext:  &tenantctx.EnvironmentContext{},
	}

	cmd := cmdlist.New(rtCtx)
	cmd.SetArgs([]string{"extra"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when extra args provided")
	}
}

func TestExecute_WithMock_PrintsWorkflowBlocks(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{threeWorkflowPage()}, 3)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{}); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(out, "gone-deleted") {
		t.Errorf("deleted workflow should be omitted by default; output:\n%s", out)
	}
	for _, want := range []string{
		"Workflows",
		"1. alpha",
		"Workflow ID:",
		"1010101010101010101010101010101010101010101010101010101010101010",
		"Owner:",
		"2020202020202020202020202020202020202020",
		"Status:",
		"ACTIVE",
		"Registry:",
		"private",
		"2. beta",
		"Workflow ID:",
		"3030303030303030303030303030303030303030303030303030303030303030",
		"Owner:",
		"4040404040404040404040404040404040404040",
		"PAUSED",
		"contract:999888777666555444333:0xabababababababababababababababababababab",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestExecute_WithMock_IncludeDeleted(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Label: "Private hosted"}},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{threeWorkflowPage()}, 3)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{IncludeDeleted: true}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "gone-deleted") || !strings.Contains(out, "DELETED") {
		t.Errorf("expected deleted workflow with --include-deleted; output:\n%s", out)
	}
}

func TestExecute_AllDeletedShowsHint(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{Registries: []*tenantctx.Registry{}},
	}

	deletedPage := []map[string]string{
		{
			"name":           "gone-deleted-only",
			"workflowId":     "7070707070707070707070707070707070707070707070707070707070707070",
			"ownerAddress":   "8080808080808080808080808080808080808080",
			"status":         "DELETED",
			"workflowSource": "private",
		},
	}
	srv := newWorkflowServer(t, [][]map[string]string{deletedPage}, 1)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	var errOut string
	captureStdout(t, func() {
		errOut = captureStderr(t, func() {
			if err := h.Execute(context.Background(), cmdlist.Inputs{}); err != nil {
				t.Fatal(err)
			}
		})
	})

	if !strings.Contains(errOut, "excluding deleted") || !strings.Contains(errOut, "--include-deleted") {
		t.Errorf("expected hint on stderr when all workflows are deleted; stderr:\n%s", errOut)
	}
}

func TestExecute_WithMock_RegistryFilter(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private", Label: "Private hosted"}},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{threeWorkflowPage()}, 3)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{RegistryFilter: "private"}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("expected only private registry row; output:\n%s", out)
	}
}

func mixedRegistriesPage() []map[string]string {
	return []map[string]string{
		{
			"name":           "onchain-wf",
			"workflowId":     "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
			"ownerAddress":   "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
			"status":         "ACTIVE",
			"workflowSource": "contract:12345678901234567890:0xcafebabe00000000000000000000000000feed",
		},
		{
			"name":           "grpc-wf",
			"workflowId":     "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3",
			"ownerAddress":   "d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4",
			"status":         "ACTIVE",
			"workflowSource": "grpc:mock-private-registry:v1",
		},
	}
}

func mixedRegistriesContext() *tenantctx.EnvironmentContext {
	chainSel := "12345678901234567890"
	addr := "0xcafebabe00000000000000000000000000feed"
	return &tenantctx.EnvironmentContext{
		Registries: []*tenantctx.Registry{
			{
				ID:            "onchain:mock-testnet",
				Label:         "mock-testnet (short)",
				ChainSelector: strPtr(chainSel),
				Address:       strPtr(addr),
			},
			{ID: "private", Label: "Private", Type: "off-chain"},
		},
	}
}

func TestExecute_RegistryFilter_MatchesContractSource(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  mixedRegistriesContext(),
	}

	srv := newWorkflowServer(t, [][]map[string]string{mixedRegistriesPage()}, 2)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{RegistryFilter: "onchain:mock-testnet"}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "onchain-wf") || strings.Contains(out, "grpc-wf") {
		t.Errorf("expected only contract-registry workflow; output:\n%s", out)
	}
}

func TestExecute_RegistryFilter_MatchesGrpcSource(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  mixedRegistriesContext(),
	}

	srv := newWorkflowServer(t, [][]map[string]string{mixedRegistriesPage()}, 2)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{RegistryFilter: "private"}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "grpc-wf") || strings.Contains(out, "onchain-wf") {
		t.Errorf("expected only grpc/private-registry workflow; output:\n%s", out)
	}
}

func TestExecute_List_ShowsRegistryIDForGrpcSource(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  mixedRegistriesContext(),
	}

	srv := newWorkflowServer(t, [][]map[string]string{mixedRegistriesPage()}, 2)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{}); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(out, "grpc:mock-private-registry:v1") {
		t.Errorf("expected resolved grpc to show context registry id, not raw API source; output:\n%s", out)
	}
	idx := strings.Index(out, "grpc-wf")
	if idx < 0 {
		t.Fatal("expected grpc-wf in output")
	}
	end := idx + 400
	if end > len(out) {
		end = len(out)
	}
	if !strings.Contains(out[idx:end], "Registry:     private") {
		t.Errorf("expected registry private near grpc-wf block; output:\n%s", out)
	}
	if strings.Contains(out[idx:end], "Address:") {
		t.Errorf("did not expect Address line for off-chain/grpc workflow; output:\n%s", out)
	}

	idxOn := strings.Index(out, "onchain-wf")
	if idxOn < 0 {
		t.Fatal("expected onchain-wf in output")
	}
	endOn := idxOn + 500
	if endOn > len(out) {
		endOn = len(out)
	}
	onChunk := out[idxOn:endOn]
	if !strings.Contains(onChunk, "onchain:mock-testnet") || !strings.Contains(onChunk, "Registry:") {
		t.Errorf("expected on-chain registry near onchain-wf block; output:\n%s", out)
	}
	if !strings.Contains(onChunk, "Address:") || !strings.Contains(onChunk, "0xcafebabe00000000000000000000000000feed") {
		t.Errorf("expected Address line for on-chain workflow; output:\n%s", onChunk)
	}
}

func orphanAndGrpcPage() []map[string]string {
	chainSel := "12345678901234567890"
	orphanAddr := "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	return []map[string]string{
		{
			"name":           "orphan-onchain",
			"workflowId":     "f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1",
			"ownerAddress":   "e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2e2",
			"status":         "ACTIVE",
			"workflowSource": "contract:" + chainSel + ":" + orphanAddr,
		},
		{
			"name":           "grpc-wf",
			"workflowId":     "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3",
			"ownerAddress":   "d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4",
			"status":         "ACTIVE",
			"workflowSource": "grpc:mock-private-registry:v1",
		},
	}
}

func TestExecute_List_UnmatchedContractShowsAPISource(t *testing.T) {
	chainSel := "12345678901234567890"
	addr := "0xcafebabe00000000000000000000000000feed"
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:mock-testnet",
					Label:         "mock",
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{orphanAndGrpcPage()}, 2)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{}); err != nil {
			t.Fatal(err)
		}
	})

	wantSource := "contract:" + chainSel + ":0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	idx := strings.Index(out, "orphan-onchain")
	if idx < 0 {
		t.Fatal("expected orphan-onchain in output")
	}
	end := idx + 500
	if end > len(out) {
		end = len(out)
	}
	chunk := out[idx:end]
	if !strings.Contains(chunk, "Registry:     "+wantSource) {
		t.Errorf("expected unmatched contract to show API workflowSource in Registry line; chunk:\n%s", chunk)
	}
	if !strings.Contains(chunk, "Address:") || !strings.Contains(chunk, "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee") {
		t.Errorf("expected Address from workflow source for orphan contract; chunk:\n%s", chunk)
	}
}

func TestExecute_RegistryFilter_PrivateExcludesUnmatchedContract(t *testing.T) {
	chainSel := "12345678901234567890"
	addr := "0xcafebabe00000000000000000000000000feed"
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:mock-testnet",
					Label:         "mock",
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{orphanAndGrpcPage()}, 2)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{RegistryFilter: "private"}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "grpc-wf") || strings.Contains(out, "orphan-onchain") {
		t.Errorf("expected private filter to include only grpc workflows resolved to private, not unmatched contract rows; output:\n%s", out)
	}
}

func TestExecute_Pagination(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{{ID: "private"}},
		},
	}

	page1 := make([]map[string]string, workflowdataclient.DefaultPageSize)
	for i := range page1 {
		page1[i] = map[string]string{
			"name":           "wf-page-batch",
			"workflowId":     "9191919191919191919191919191919191919191919191919191919191919191",
			"ownerAddress":   "9292929292929292929292929292929292929292",
			"status":         "ACTIVE",
			"workflowSource": "private",
		}
	}
	page2 := []map[string]string{
		{
			"name":           "wf-page-tail-1",
			"workflowId":     "9393939393939393939393939393939393939393939393939393939393939393",
			"ownerAddress":   "9292929292929292929292929292929292929292",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
		{
			"name":           "wf-page-tail-2",
			"workflowId":     "9494949494949494949494949494949494949494949494949494949494949494",
			"ownerAddress":   "9292929292929292929292929292929292929292",
			"status":         "ACTIVE",
			"workflowSource": "private",
		},
	}

	total := workflowdataclient.DefaultPageSize + 2
	srv := newWorkflowServer(t, [][]map[string]string{page1, page2}, total)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	out := captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{}); err != nil {
			t.Fatal(err)
		}
	})

	wantRows := workflowdataclient.DefaultPageSize + 2
	if got := strings.Count(out, "9292929292929292929292929292929292929292"); got < wantRows {
		t.Errorf("expected at least %d owner cells, got %d in:\n%s", wantRows, got, out)
	}
}

func TestExecute_JSONOutput_WritesFile(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{threeWorkflowPage()}, 3)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	outFile := t.TempDir() + "/workflows.json"
	captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{OutputPath: outFile}); err != nil {
			t.Fatal(err)
		}
	})

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected JSON file to be written: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON written: %v\n%s", err, data)
	}

	// Deleted workflow is excluded (includeDeleted=false).
	if len(result) != 2 {
		t.Fatalf("expected 2 workflows in JSON (deleted excluded), got %d", len(result))
	}

	names := []string{result[0]["name"].(string), result[1]["name"].(string)}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("unexpected workflow names in JSON: %v", names)
	}
	if result[0]["registry"] != "private" {
		t.Errorf("expected registry=private for alpha, got %v", result[0]["registry"])
	}
	if result[0]["status"] != "ACTIVE" {
		t.Errorf("expected status=ACTIVE for alpha, got %v", result[0]["status"])
	}
}

func TestExecute_JSONOutput_IncludeDeleted(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	srv := newWorkflowServer(t, [][]map[string]string{threeWorkflowPage()}, 3)
	defer srv.Close()
	h := newHandlerWithServer(t, rtCtx, srv)

	outFile := t.TempDir() + "/workflows.json"
	captureStdout(t, func() {
		if err := h.Execute(context.Background(), cmdlist.Inputs{IncludeDeleted: true, OutputPath: outFile}); err != nil {
			t.Fatal(err)
		}
	})

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected JSON file to be written: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON written: %v\n%s", err, data)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 workflows in JSON (include-deleted), got %d", len(result))
	}

	statuses := make([]string, len(result))
	for i, r := range result {
		statuses[i] = r["status"].(string)
	}
	if statuses[2] != "DELETED" {
		t.Errorf("expected last workflow to be DELETED, got %v", statuses[2])
	}
}
