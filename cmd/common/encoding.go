package common

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andybalholm/brotli"
)

// wasmMagic is the first four bytes of every valid WASM binary ("\0asm").
var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

// CompressBrotli applies Brotli compression to the given data.
func CompressBrotli(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := brotli.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecompressBrotli decompresses Brotli-compressed data.
func DecompressBrotli(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

// EncodeBase64ToFile base64-encodes data and writes the result to the given path.
func EncodeBase64ToFile(data []byte, path string) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	return os.WriteFile(path, []byte(encoded), 0666) //nolint:gosec
}

// EnsureOutputExtension appends .wasm, .br, and/or .b64 suffixes as needed so the
// returned path always ends with ".wasm.br.b64".
func EnsureOutputExtension(outputPath string) string {
	if !strings.HasSuffix(outputPath, ".b64") {
		if !strings.HasSuffix(outputPath, ".br") {
			if !strings.HasSuffix(outputPath, ".wasm") {
				outputPath += ".wasm"
			}
			outputPath += ".br"
		}
		outputPath += ".b64"
	}
	return outputPath
}

// EnsureWasmExtension appends ".wasm" if the path doesn't already end with it.
func EnsureWasmExtension(outputPath string) string {
	if !strings.HasSuffix(outputPath, ".wasm") {
		outputPath += ".wasm"
	}
	return outputPath
}

// IsRawWasm returns true if data starts with the WASM magic number ("\0asm").
func IsRawWasm(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], wasmMagic)
}

// EnsureBrotliBase64 returns data in brotli-compressed, base64-encoded form.
// If the input is raw WASM (starts with \0asm), it compresses and encodes.
// Otherwise it assumes the data is already in br64 form and returns it as-is.
func EnsureBrotliBase64(data []byte) ([]byte, error) {
	if !IsRawWasm(data) {
		return data, nil
	}
	compressed, err := CompressBrotli(data)
	if err != nil {
		return nil, fmt.Errorf("brotli compress: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(compressed)
	return []byte(encoded), nil
}

// EnsureRawWasm returns raw WASM bytes. If data is already raw WASM (starts
// with \0asm), it is returned as-is. Otherwise the data is assumed to be
// base64-encoded brotli-compressed WASM and is decoded then decompressed.
func EnsureRawWasm(data []byte) ([]byte, error) {
	if IsRawWasm(data) {
		return data, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	raw, err := DecompressBrotli(decoded)
	if err != nil {
		return nil, fmt.Errorf("brotli decompress: %w", err)
	}
	return raw, nil
}
