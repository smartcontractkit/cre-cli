package limits

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/registrytype"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

type fakeExecutor struct {
	response string
	err      error
}

func (f *fakeExecutor) Execute(_ context.Context, _ *graphql.Request, resp any) error {
	if f.response != "" {
		if err := json.Unmarshal([]byte(f.response), resp); err != nil {
			return err
		}
	}
	return f.err
}

func strPtr(s string) *string { return &s }

func newTestRuntime(registries []*tenantctx.Registry) *runtime.Context {
	logger := zerolog.New(io.Discard)
	return &runtime.Context{
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{EnvName: "STAGING"},
		TenantContext:  &tenantctx.EnvironmentContext{Registries: registries},
	}
}

func runCommand(t *testing.T, rtCtx *runtime.Context, executor GraphQLExecutor) (string, error) {
	t.Helper()

	cmd := newWithExecutor(rtCtx, executor)
	cmd.SetArgs([]string{})

	oldStdout, oldStderr := os.Stdout, os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	// ui.Warning writes to stderr, so both streams go to the same pipe to keep the assertion
	// order-independent.
	os.Stdout, os.Stderr = write, write
	defer func() { os.Stdout, os.Stderr = oldStdout, oldStderr }()

	runErr := cmd.Execute()
	if closeErr := write.Close(); closeErr != nil {
		t.Fatalf("close pipe: %v", closeErr)
	}
	out, readErr := io.ReadAll(read)
	if readErr != nil {
		t.Fatalf("read command output: %v", readErr)
	}

	return string(out), runErr
}

const onchainRegistryFixture = `{
  "registryQuotas": {
    "privateRegistry": null,
    "onchainRegistries": [{
      "registryId": "ethereum-mainnet",
      "chainSelector": "5009297550715157269",
      "address": "0x4ac54353fa4fa961afcc5ec4b118596d3305e7e5",
      "linkedOwners": {"linked": ["0xaaa"], "pending": [], "limit": 1},
      "owners": [{"owner": "0xaaa", "workflows": [
        {"donFamily": "zone-a", "active": 1, "limit": 3},
        {"donFamily": "zone-b", "active": 4, "limit": 0}
      ]}]
    }]
  }
}`

func TestLimits_JoinsOnchainQuotaByChainSelector(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:            "onchain:ethereum-mainnet",
		Label:         "Ethereum Mainnet (0x4Ac5…E7e5)",
		Type:          string(registrytype.OnChain),
		ChainSelector: strPtr("5009297550715157269"),
		Address:       strPtr("0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"),
	}})

	out, err := runCommand(t, rtCtx, &fakeExecutor{response: onchainRegistryFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Registry limits") {
		t.Fatalf("expected the limits header, got:\n%s", out)
	}
	if !strings.Contains(out, "1 / 3 used") {
		t.Fatalf("expected the quota to be attached to the registry, got:\n%s", out)
	}
	if !strings.Contains(out, "1 / 1 used") {
		t.Fatalf("expected the linked-owner quota, got:\n%s", out)
	}
}

func TestLimits_RendersPrivateWorkflows(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:    "private",
		Label: "Private (Chainlink-hosted)",
		Type:  string(registrytype.OffChain),
	}})

	fixture := `{
	  "registryQuotas": {
	    "privateRegistry": {
	      "registryId": "private",
	      "workflows": [{"donFamily": "zone-a", "active": 1, "limit": 3}]
	    },
	    "onchainRegistries": []
	  }
	}`

	out, err := runCommand(t, rtCtx, &fakeExecutor{response: fixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Workflows:") || !strings.Contains(out, "1 / 3 used") {
		t.Fatalf("expected private registry workflows, got:\n%s", out)
	}
}

func TestLimits_GatedFamilyRendersNoHeadroom(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:            "onchain:ethereum-mainnet",
		Type:          string(registrytype.OnChain),
		ChainSelector: strPtr("5009297550715157269"),
	}})

	out, err := runCommand(t, rtCtx, &fakeExecutor{response: onchainRegistryFixture})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "4 / 0 used") || !strings.Contains(out, "no headroom") {
		t.Fatalf("expected the gated family to render with no headroom, got:\n%s", out)
	}
	if strings.Contains(out, "-4") {
		t.Fatalf("remaining must clamp at zero, got:\n%s", out)
	}
}

// A chain read failure nulls only the owners section; the command still prints what it has.
func TestLimits_PartialQuotasWarnAndSucceed(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:            "onchain:ethereum-mainnet",
		Type:          string(registrytype.OnChain),
		ChainSelector: strPtr("5009297550715157269"),
	}})

	partial := `{
	  "registryQuotas": {
	    "privateRegistry": null,
	    "onchainRegistries": [{
	      "registryId": "ethereum-mainnet",
	      "chainSelector": "5009297550715157269",
	      "address": "0x4ac5",
	      "linkedOwners": {"linked": ["0xaaa"], "pending": [], "limit": 2},
	      "owners": null
	    }]
	  }
	}`

	out, err := runCommand(t, rtCtx, &fakeExecutor{
		response: partial,
		err:      errors.New("cre api error: on-chain registry temporarily unavailable"),
	})
	if err != nil {
		t.Fatalf("a partial result must not fail the command: %v", err)
	}
	if !strings.Contains(out, "1 / 2 used") {
		t.Fatalf("expected linked owners to render, got:\n%s", out)
	}
	if !strings.Contains(out, "Registry limits are incomplete") ||
		!strings.Contains(out, "on-chain registry temporarily unavailable") {
		t.Fatalf("expected a warning with the server message, got:\n%s", out)
	}
}

func TestLimits_MissingPermissionFails(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:   "private",
		Type: string(registrytype.OffChain),
	}})

	out, err := runCommand(t, rtCtx, &fakeExecutor{
		err: errors.New("cre api error: unauthorized: for permission platform:limits:read"),
	})
	if err == nil {
		t.Fatalf("a limits command without permission must fail, got output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "platform:limits:read") {
		t.Fatalf("expected the authorization message, got: %v", err)
	}
}

func TestLimits_NoRegistries(t *testing.T) {
	rtCtx := newTestRuntime(nil)

	out, err := runCommand(t, rtCtx, &fakeExecutor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No registries found") {
		t.Fatalf("expected the no-registries warning, got:\n%s", out)
	}
}

func TestLimits_NoQuotaDataWarns(t *testing.T) {
	rtCtx := newTestRuntime([]*tenantctx.Registry{{
		ID:    "private",
		Label: "Private (Chainlink-hosted)",
		Type:  string(registrytype.OffChain),
	}})

	out, err := runCommand(t, rtCtx, &fakeExecutor{response: `{"registryQuotas":{"privateRegistry":null,"onchainRegistries":[]}}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No registry limits available") {
		t.Fatalf("expected the empty-quota warning, got:\n%s", out)
	}
}
