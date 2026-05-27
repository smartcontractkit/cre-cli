package common

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRawWasm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid wasm magic", []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00}, true},
		{"just the magic", []byte{0x00, 0x61, 0x73, 0x6d}, true},
		{"not wasm", []byte("hello world"), false},
		{"too short", []byte{0x00, 0x61}, false},
		{"empty", nil, false},
		{"base64 text", []byte("SGVsbG8gV29ybGQ="), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsRawWasm(tt.data))
		})
	}
}

func TestEnsureBrotliBase64(t *testing.T) {
	t.Parallel()

	t.Run("raw wasm gets compressed and encoded", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("test wasm payload")...)

		result, err := EnsureBrotliBase64(raw)
		require.NoError(t, err)

		decoded, err := base64.StdEncoding.DecodeString(string(result))
		require.NoError(t, err)

		decompressed, err := DecompressBrotli(decoded)
		require.NoError(t, err)
		assert.Equal(t, raw, decompressed)
	})

	t.Run("non-wasm data passes through unchanged", func(t *testing.T) {
		t.Parallel()
		br64Data := []byte("already-processed-base64-data")

		result, err := EnsureBrotliBase64(br64Data)
		require.NoError(t, err)
		assert.Equal(t, br64Data, result)
	})
}

func TestEnsureRawWasm(t *testing.T) {
	t.Parallel()

	t.Run("raw wasm passes through unchanged", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("test wasm payload")...)

		result, err := EnsureRawWasm(raw)
		require.NoError(t, err)
		assert.Equal(t, raw, result)
	})

	t.Run("br64 data gets decoded and decompressed", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("test wasm payload")...)

		compressed, err := CompressBrotli(raw)
		require.NoError(t, err)
		br64 := []byte(base64.StdEncoding.EncodeToString(compressed))

		result, err := EnsureRawWasm(br64)
		require.NoError(t, err)
		assert.Equal(t, raw, result)
	})

	t.Run("invalid base64 returns error", func(t *testing.T) {
		t.Parallel()
		_, err := EnsureRawWasm([]byte("not!valid!base64!!!"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base64 decode")
	})
}

func TestEnsureWasmExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no extension", "./my-binary", "./my-binary.wasm"},
		{"already .wasm", "./my-binary.wasm", "./my-binary.wasm"},
		{"different extension", "./my-binary.bin", "./my-binary.bin.wasm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, EnsureWasmExtension(tt.input))
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("round-trip test data")...)

	br64, err := EnsureBrotliBase64(raw)
	require.NoError(t, err)

	result, err := EnsureRawWasm(br64)
	require.NoError(t, err)
	assert.Equal(t, raw, result)
}
