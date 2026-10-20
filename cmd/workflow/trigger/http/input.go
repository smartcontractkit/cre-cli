package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// parseInput accepts inline JSON or @<path> and returns a validated JSON object or array.
func parseInput(raw, invocationDir string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	source := "--input"

	if path, ok := strings.CutPrefix(raw, "@"); ok {
		resolved := resolveFromInvocation(strings.TrimSpace(path), invocationDir)
		data, err := os.ReadFile(resolved) // #nosec G304 -- path is an explicit CLI argument
		if err != nil {
			return nil, fmt.Errorf("failed to read --input file: %w", err)
		}
		raw = string(bytes.TrimSpace(data))
		source = fmt.Sprintf("--input file %s", resolved)
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", source, err)
	}
	// The Gateway rejects primitives, so fail before spending a request on it.
	switch decoded.(type) {
	case map[string]any, []any:
	default:
		return nil, fmt.Errorf("%s must be a JSON object or array, got %s", source, jsonKind(decoded))
	}

	return json.RawMessage(raw), nil
}

// resolveFromInvocation anchors relative paths at the directory the user ran the CLI from,
// because the process cwd is moved into the workflow folder before handlers run.
func resolveFromInvocation(path, invocationDir string) string {
	if filepath.IsAbs(path) || invocationDir == "" {
		return path
	}
	return filepath.Join(invocationDir, path)
}

func jsonKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	default:
		return "a number"
	}
}

// normalizeWorkflowID returns the 0x-prefixed lower-case form, which is the only casing the Gateway accepts.
func normalizeWorkflowID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	id = strings.TrimPrefix(strings.TrimPrefix(id, "0x"), "0X")
	if !workflowresolve.LooksLikeWorkflowID(id) {
		return "", fmt.Errorf("invalid workflow ID %q: expected 64 hex characters with an optional 0x prefix", raw)
	}
	return "0x" + strings.ToLower(id), nil
}
