package list_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/cmd/registry/list"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func strPtr(s string) *string { return &s }

func TestList_NoTenantContext(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  nil,
	}

	cmd := list.New(rtCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when TenantContext is nil")
	}
	if !strings.Contains(err.Error(), "user context not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestList_EmptyRegistries(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{},
		},
	}

	cmd := list.New(rtCtx)
	cmd.SetArgs([]string{})

	// suppress stderr (ui.Warning writes there)
	oldStderr := os.Stderr
	os.Stderr, _ = os.Open(os.DevNull)
	defer func() { os.Stderr = oldStderr }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestList_OnChainAndOffChain(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:      "onchain:ethereum-testnet-sepolia",
					Label:   "ethereum-testnet-sepolia (0xaE55...1135)",
					Type:    "on-chain",
					Address: strPtr("0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"),
				},
				{
					ID:    "private",
					Label: "Private (Chainlink-hosted)",
					Type:  "off-chain",
				},
			},
		},
	}

	cmd := list.New(rtCtx)
	cmd.SetArgs([]string{})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	if err := cmd.Execute(); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	for _, want := range []string{
		"Registries available to your organization",
		"onchain:ethereum-testnet-sepolia",
		"on-chain",
		"0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135",
		"private",
		"off-chain",
		"Private (Chainlink-hosted)",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q; full output:\n%s", want, output)
		}
	}
}

func TestList_OffChainNoAddress(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext: &tenantctx.EnvironmentContext{
			Registries: []*tenantctx.Registry{
				{
					ID:    "private",
					Label: "Private",
					Type:  "off-chain",
				},
			},
		},
	}

	cmd := list.New(rtCtx)
	cmd.SetArgs([]string{})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	if err := cmd.Execute(); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if strings.Contains(output, "Addr:") {
		t.Errorf("expected no Addr line for off-chain registry; output:\n%s", output)
	}
}

func TestList_RejectsArgs(t *testing.T) {
	logger := zerolog.New(io.Discard)
	rtCtx := &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{},
		TenantContext:  &tenantctx.EnvironmentContext{},
	}

	cmd := list.New(rtCtx)
	cmd.SetArgs([]string{"extra"})

	// cobra prints usage to stderr on arg errors; suppress
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when extra args provided")
	}
}
