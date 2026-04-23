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

	cmdlist "github.com/smartcontractkit/cre-cli/cmd/workflow/list"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func strPtr(s string) *string { return &s }

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
	h := cmdlist.NewHandlerWithClient(rtCtx, fake)

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
	h := cmdlist.NewHandlerWithClient(rtCtx, fake)

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
	h := cmdlist.NewHandlerWithClient(rtCtx, deletedOnly)

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
					"name":           "gone-deleted-only",
					"workflowId":     "7070707070707070707070707070707070707070707070707070707070707070",
					"ownerAddress":   "8080808080808080808080808080808080808080",
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
	h := cmdlist.NewHandlerWithClient(rtCtx, fake)

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
	chainSel := "12345678901234567890"
	addr := "0xcafebabe00000000000000000000000000feed"
	body, err := json.Marshal(map[string]any{
		"workflows": map[string]any{
			"count": 2,
			"data": []map[string]string{
				{
					"name":           "onchain-wf",
					"workflowId":     "a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1",
					"ownerAddress":   "b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",
					"status":         "ACTIVE",
					"workflowSource": "contract:" + chainSel + ":" + addr,
				},
				{
					"name":           "grpc-wf",
					"workflowId":     "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3",
					"ownerAddress":   "d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4",
					"status":         "ACTIVE",
					"workflowSource": "grpc:mock-private-registry:v1",
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
					ID:    "onchain:mock-testnet",
					Label: "mock-testnet (short)",
					// type often omitted in user context; matching uses address + chain selector
					ChainSelector: strPtr(chainSel),
					Address:       strPtr(addr),
				},
				{ID: "private", Label: "Private", Type: "off-chain"},
			},
		},
	}

	h := cmdlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = h.Execute(context.Background(), "onchain:mock-testnet", false)
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

	h := cmdlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

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

	h := cmdlist.NewHandlerWithClient(rtCtx, &fakeGQLMixedRegistries{})

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

	// Resolved grpc maps to context "private"; unresolved grpc would print the raw API source.
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
		t.Errorf("expected registry private (as in cre registry list) near grpc-wf block; output:\n%s", out)
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
	if !strings.Contains(onChunk, "onchain:mock-testnet") ||
		!strings.Contains(onChunk, "Registry:") {
		t.Errorf("expected on-chain registry as in cre registry list near onchain-wf block; output:\n%s", out)
	}
	if !strings.Contains(onChunk, "Address:") ||
		!strings.Contains(onChunk, "0xcafebabe00000000000000000000000000feed") {
		t.Errorf("expected full registry Address line for on-chain workflow; output:\n%s", onChunk)
	}
}

// fakeGQLOrphanContractAndGrpc: contract address not in user context + one grpc workflow.
type fakeGQLOrphanContractAndGrpc struct{}

func (f *fakeGQLOrphanContractAndGrpc) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	chainSel := "12345678901234567890"
	orphanAddr := "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	body, err := json.Marshal(map[string]any{
		"workflows": map[string]any{
			"count": 2,
			"data": []map[string]string{
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
			},
		},
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
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

	h := cmdlist.NewHandlerWithClient(rtCtx, &fakeGQLOrphanContractAndGrpc{})

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
	if !strings.Contains(chunk, "Address:") ||
		!strings.Contains(chunk, "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee") {
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

	h := cmdlist.NewHandlerWithClient(rtCtx, &fakeGQLOrphanContractAndGrpc{})

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

	if !strings.Contains(out, "grpc-wf") || strings.Contains(out, "orphan-onchain") {
		t.Errorf("expected private filter to include only grpc workflows resolved to private, not unmatched contract rows; output:\n%s", out)
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
		data := make([]map[string]string, cmdlist.DefaultPageSize)
		for i := range data {
			data[i] = map[string]string{
				"name":           "wf-page-batch",
				"workflowId":     "9191919191919191919191919191919191919191919191919191919191919191",
				"ownerAddress":   "9292929292929292929292929292929292929292",
				"status":         "ACTIVE",
				"workflowSource": "private",
			}
		}
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": cmdlist.DefaultPageSize + 2,
				"data":  data,
			},
		})
	case 2:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": cmdlist.DefaultPageSize + 2,
				"data": []map[string]string{
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
				},
			},
		})
	default:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{"count": cmdlist.DefaultPageSize + 2, "data": []any{}},
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
	h := cmdlist.NewHandlerWithClient(rtCtx, fake)

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

	wantRows := cmdlist.DefaultPageSize + 2
	if got := strings.Count(out, "9292929292929292929292929292929292929292"); got < wantRows {
		t.Errorf("expected at least %d owner cells, got %d in:\n%s", wantRows, got, out)
	}
	if fake.call != 2 {
		t.Errorf("expected 2 GQL calls, got %d", fake.call)
	}
}
