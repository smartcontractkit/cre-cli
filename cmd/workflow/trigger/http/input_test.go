package http

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInput(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "payload.json"), []byte("\n {\"from\": \"file\"}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scalar.json"), []byte(`"just a string"`), 0o600))

	tests := []struct {
		name    string
		raw     string
		invDir  string
		want    string
		wantErr string
	}{
		{name: "empty object", raw: "{}", want: "{}"},
		{name: "object", raw: ` {"a": 1} `, want: `{"a": 1}`},
		{name: "array", raw: `[1, 2]`, want: `[1, 2]`},
		{name: "relative file resolves against invocation dir", raw: "@payload.json", invDir: dir, want: `{"from": "file"}`},
		{name: "absolute file", raw: "@" + filepath.Join(dir, "payload.json"), want: `{"from": "file"}`},
		{name: "invalid JSON", raw: `{oops`, wantErr: "--input is not valid JSON"},
		{name: "empty", raw: "  ", wantErr: "--input is not valid JSON"},
		{name: "string primitive", raw: `"x"`, wantErr: "must be a JSON object or array, got a string"},
		{name: "number primitive", raw: `7`, wantErr: "must be a JSON object or array, got a number"},
		{name: "boolean primitive", raw: `true`, wantErr: "must be a JSON object or array, got a boolean"},
		{name: "null", raw: `null`, wantErr: "must be a JSON object or array, got null"},
		{name: "missing file", raw: "@nope.json", invDir: dir, wantErr: "failed to read --input file"},
		{name: "file with primitive", raw: "@scalar.json", invDir: dir, wantErr: "must be a JSON object or array"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInput(tt.raw, tt.invDir)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestNormalizeWorkflowID(t *testing.T) {
	const hexID = "00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "bare hex gains prefix", raw: hexID, want: "0x" + hexID},
		{name: "prefixed hex", raw: "0x" + hexID, want: "0x" + hexID},
		{name: "upper case is lowered", raw: "0X00DA21B8B3E117E31F3A3E8A0795225CBDE6C00283A84395117669691F2B7856", want: "0x" + hexID},
		{name: "whitespace is trimmed", raw: "  " + hexID + "\n", want: "0x" + hexID},
		{name: "too short", raw: "0x1234", wantErr: true},
		{name: "not hex", raw: "0x" + hexID[:62] + "zz", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWorkflowID(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
