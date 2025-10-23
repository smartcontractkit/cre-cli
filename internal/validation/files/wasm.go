package files

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/go-playground/validator/v10"
)

func IsValidWASM(fl validator.FieldLevel) bool {
	field := fl.Field()

	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}
	path := field.String()

	// Check if the file has a valid extension (.wasm or .wasm.br)
	if !strings.HasSuffix(path, ".wasm") && !strings.HasSuffix(path, ".wasm.br") {
		return false
	}

	// Check if the file exists
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.IsDir() {
		return false
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Handle decompression for Brotli-compressed files
	var reader io.Reader
	if strings.HasSuffix(path, ".wasm.br") {
		reader = brotli.NewReader(file)
	} else {
		reader = file
	}

	// Read the first 4 bytes (WASM magic number)
	// https://webassembly.github.io/spec/core/binary/modules.html#binary-module
	header := make([]byte, 4)
	_, err = reader.Read(header)
	if err != nil {
		return false
	}

	// Validate the magic number and version
	expectedHeader := []byte{0x00, 0x61, 0x73, 0x6D} // "\0asm" in binary
	bytes.Equal(header, expectedHeader)
	return bytes.Equal(header, expectedHeader)
}
