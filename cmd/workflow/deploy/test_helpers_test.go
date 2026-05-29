package deploy

import (
	"context"
	"io"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// newTestHandler returns a handler suitable for unit tests that call handler
// methods directly instead of going through Execute(). It pre-sets execCtx so
// cancellation-aware code paths behave like a normal CLI invocation.
func newTestHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	h := newHandler(ctx, stdin)
	h.execCtx = context.Background()
	return h
}
