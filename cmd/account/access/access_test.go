package access_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/cmd/account/access"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func TestHandlerExecute_HasAccess(t *testing.T) {
	// API key auth type always returns HasAccess: true
	creds := &credentials.Credentials{
		AuthType: "api-key",
		APIKey:   "test-key",
	}
	logger := zerolog.New(io.Discard)
	envSet := &environments.EnvironmentSet{}

	rtCtx := &runtime.Context{
		Credentials:    creds,
		Logger:         &logger,
		EnvironmentSet: envSet,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	h := access.NewHandler(rtCtx)
	err := h.Execute(context.Background())

	w.Close()
	os.Stdout = oldStdout
	var output strings.Builder
	_, _ = io.Copy(&output, r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := output.String()
	expectedSnippets := []string{
		"deployment access enabled",
		"cre workflow deploy",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(out, snippet) {
			t.Errorf("output missing %q; full output:\n%s", snippet, out)
		}
	}
}

func TestHandlerExecute_NoTokens(t *testing.T) {
	// Bearer auth with no tokens should return an error from GetDeploymentAccessStatus
	creds := &credentials.Credentials{
		AuthType: "bearer",
	}
	logger := zerolog.New(io.Discard)
	envSet := &environments.EnvironmentSet{}

	rtCtx := &runtime.Context{
		Credentials:    creds,
		Logger:         &logger,
		EnvironmentSet: envSet,
	}

	h := access.NewHandler(rtCtx)
	err := h.Execute(context.Background())

	if err == nil {
		t.Fatal("expected error for missing tokens, got nil")
	}
	if !strings.Contains(err.Error(), "failed to check deployment access") {
		t.Errorf("expected 'failed to check deployment access' error, got: %v", err)
	}
}
