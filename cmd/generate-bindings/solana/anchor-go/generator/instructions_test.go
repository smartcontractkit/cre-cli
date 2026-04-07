//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"strings"
	"testing"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenInstructionsZeroArgsAlwaysWritesDiscriminator(t *testing.T) {
	idlData := &idl.Idl{
		Instructions: []idl.IdlInstruction{
			{
				Name: "ping",
				Discriminator: idl.IdlDiscriminator{1, 2, 3, 4, 5, 6, 7, 8},
				Args: []idl.IdlField{},
			},
		},
	}

	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	outputFile, err := gen.gen_instructions()
	require.NoError(t, err)
	require.NotNil(t, outputFile)

	code := outputFile.File.GoString()

	assert.Contains(t, code, "NewBorshEncoder",
		"zero-arg instruction builder must allocate an encoder for the discriminator")
	assert.Contains(t, code, "WriteBytes",
		"zero-arg instruction builder must write the discriminator bytes")
	assert.NotContains(t, code, "nil, // No arguments to encode",
		"zero-arg instruction must not pass nil as instruction data")

	// The generated code must reference buf__.Bytes() so the discriminator is sent.
	builderIdx := strings.Index(code, "func NewPingInstruction")
	require.Greater(t, builderIdx, 0, "expected to find NewPingInstruction in generated code")
	builderSnippet := code[builderIdx:]
	assert.Contains(t, builderSnippet, "buf__.Bytes()",
		"zero-arg instruction must pass buf__.Bytes() (containing the discriminator) as instruction data")
}
