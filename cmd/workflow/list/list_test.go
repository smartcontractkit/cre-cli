package list_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	workflowlist "github.com/smartcontractkit/cre-cli/cmd/workflow/list"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// Must match workflowListPageSize in list.go.
const testWorkflowListPageSize = 100

func strPtr(s string) *string { return &s }

func TestNew_NoTenantContext(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  nil,
	}

	cmd := workflowlist.New(rtCtx)
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

	cmd := workflowlist.New(rtCtx)
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

	cmd := workflowlist.New(rtCtx)
	cmd.SetArgs([]string{"--registry", "nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown registry")
	}
	if !strings.Contains(err.Error(), "not found in context.yaml") {
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

	cmd := workflowlist.New(rtCtx)
	cmd.SetArgs([]string{"extra"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when extra args provided")
	}
}

type fakeGQL struct {
	call int
}

func (f *fakeGQL) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	f.call++
	var body []byte
	var err error

	switch f.call {
	case 1:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": 3,
				"data": []map[string]string{
					{
						"name":           "alpha",
						"workflowId":     "0xaaa",
						"ownerAddress":   "0xowner1",
						"status":         "ACTIVE",
						"workflowSource": "private",
					},
					{
						"name":           "beta",
						"workflowId":     "0xbbb",
						"ownerAddress":   "0xowner2",
						"status":         "PAUSED",
						"workflowSource": "other",
					},
					{
						"name":           "gone-deleted",
						"workflowId":     "0xccc",
						"ownerAddress":   "0xowner3",
						"status":         "DELETED",
						"workflowSource": "private",
					},
				},
			},
		})
	default:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": 3,
				"data":  []any{},
			},
		})
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
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

	fake := &fakeGQL{}
	h := workflowlist.NewHandlerWithClient(rtCtx, fake)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if strings.Contains(out, "gone-deleted") {
		t.Errorf("deleted workflow should be omitted by default; output:\n%s", out)
	}
	for _, want := range []string{
		"Workflows",
		"1. alpha",
		"Workflow ID:",
		"0xaaa",
		"Owner:",
		"0xowner1",
		"Status:",
		"ACTIVE",
		"Registry:",
		"Private hosted",
		"2. beta",
		"0xbbb",
		"0xowner2",
		"PAUSED",
		"other",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if fake.call != 1 {
		t.Errorf("expected single GQL page, got %d calls", fake.call)
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

	fake := &fakeGQL{}
	h := workflowlist.NewHandlerWithClient(rtCtx, fake)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "", true)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

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

	deletedOnly := &fakeGQLDeletedOnly{}
	h := workflowlist.NewHandlerWithClient(rtCtx, deletedOnly)

	oldStdout := os.Stdout
	sr, sw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = sw

	oldStderr := os.Stderr
	er, ew, err := os.Pipe()
	if err != nil {
		sw.Close()
		os.Stdout = oldStdout
		t.Fatal(err)
	}
	os.Stderr = ew

	err = h.Execute(context.Background(), "", false)
	sw.Close()
	ew.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	if err != nil {
		t.Fatal(err)
	}

	var stderrBuf strings.Builder
	_, _ = io.Copy(&stderrBuf, er)
	errOut := stderrBuf.String()

	if !strings.Contains(errOut, "excluding deleted") || !strings.Contains(errOut, "--include-deleted") {
		t.Errorf("expected hint on stderr when all workflows are deleted; stderr:\n%s", errOut)
	}

	_, _ = io.Copy(io.Discard, sr)
}

type fakeGQLDeletedOnly struct{}

func (f *fakeGQLDeletedOnly) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	body, err := json.Marshal(map[string]any{
		"workflows": map[string]any{
			"count": 1,
			"data": []map[string]string{
				{
					"name":           "x",
					"workflowId":     "0x1",
					"ownerAddress":   "0x2",
					"status":         "DELETED",
					"workflowSource": "private",
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
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

	fake := &fakeGQL{}
	h := workflowlist.NewHandlerWithClient(rtCtx, fake)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "private", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("expected only private registry row; output:\n%s", out)
	}
}

// fakeGQLMixedRegistries returns one on-chain (contract:…) and one grpc workflow.
type fakeGQLMixedRegistries struct{}

func (f *fakeGQLMixedRegistries) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	chainSel := "16015286601757825753"
	addr := "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"
	body, err := json.Marshal(map[string]any{
		"workflows": map[string]any{
			"count": 2,
			"data": []map[string]string{
				{
					"name":           "onchain-wf",
					"workflowId":     "00aa",
					"ownerAddress":   "0xbb",
					"status":         "ACTIVE",
					"workflowSource": "contract:" + chainSel + ":" + addr,
				},
				{
					"name":           "grpc-wf",
					"workflowId":     "00cc",
					"ownerAddress":   "0xdd",
					"status":         "ACTIVE",
					"workflowSource": "grpc:private-grpc-registry:v1",
				},
			},
		},
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
}

func TestExecute_RegistryFilter_MatchesContractSource(t *testing.T) {
	chainSel := "16015286601757825753"
	addr := "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:    "onchain:ethereum-testnet-sepolia",
					Label: "ethereum-testnet-sepolia (0xaE55...1135)",
					// type often omitted in context.yaml; matching uses address + chain selector
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private", Type: "off-chain"},
			},
		},
	}

	h := workflowlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "onchain:ethereum-testnet-sepolia", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "onchain-wf") || strings.Contains(out, "grpc-wf") {
		t.Errorf("expected only contract-registry workflow; output:\n%s", out)
	}
}

func TestExecute_RegistryFilter_MatchesGrpcSource(t *testing.T) {
	chainSel := "16015286601757825753"
	addr := "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:ethereum-testnet-sepolia",
					Label:         "sepolia",
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	h := workflowlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "private", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "grpc-wf") || strings.Contains(out, "onchain-wf") {
		t.Errorf("expected only grpc/private-registry workflow; output:\n%s", out)
	}
}

func TestExecute_List_ShowsRegistryIDForGrpcSource(t *testing.T) {
	chainSel := "16015286601757825753"
	addr := "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		Credentials:    &credentials.Credentials{},
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:            "onchain:ethereum-testnet-sepolia",
					Label:         "sepolia",
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private hosted"},
			},
		},
	}

	h := workflowlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if strings.Contains(out, "grpc:private-grpc-registry:v1") {
		t.Errorf("expected grpc source mapped to registry id, not raw API source; output:\n%s", out)
	}
	idx := strings.Index(out, "grpc-wf")
	if idx < 0 {
		t.Fatal("expected grpc-wf in output")
	}
	end := idx + 400
	if end > len(out) {
		end = len(out)
	}
	if !strings.Contains(out[idx:end], "private") {
		t.Errorf("expected registry id private near grpc-wf block; output:\n%s", out)
	}
}

type pagingFake struct {
	call int
}

func (f *pagingFake) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	f.call++
	var body []byte
	var err error
	switch f.call {
	case 1:
		data := make([]map[string]string, testWorkflowListPageSize)
		for i := range data {
			data[i] = map[string]string{
				"name":           "wf",
				"workflowId":     "0x1",
				"ownerAddress":   "0xo",
				"status":         "ACTIVE",
				"workflowSource": "private",
			}
		}
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": testWorkflowListPageSize + 2,
				"data":  data,
			},
		})
	case 2:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": testWorkflowListPageSize + 2,
				"data": []map[string]string{
					{
						"name":           "last",
						"workflowId":     "0x2",
						"ownerAddress":   "0xo",
						"status":         "ACTIVE",
						"workflowSource": "private",
					},
					{
						"name":           "last2",
						"workflowId":     "0x3",
						"ownerAddress":   "0xo",
						"status":         "ACTIVE",
						"workflowSource": "private",
					},
				},
			},
		})
	default:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{"count": testWorkflowListPageSize + 2, "data": []any{}},
		})
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
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

	fake := &pagingFake{}
	h := workflowlist.NewHandlerWithClient(rtCtx, fake)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "", false)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	wantRows := testWorkflowListPageSize + 2
	if got := strings.Count(out, "0xo"); got < wantRows {
		t.Errorf("expected at least %d owner cells, got %d in:\n%s", wantRows, got, out)
	}
	if fake.call != 2 {
		t.Errorf("expected 2 GQL calls, got %d", fake.call)
	}
}
