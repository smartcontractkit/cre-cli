//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"strings"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collidingNamedFields is an IDL shape where two distinct field names normalize to the
// same Go identifier via tools.ToCamelUpper (foo_bar and fooBar -> FooBar). Struct
// generation deconflicts these as FooBar and FooBar1; marshal/unmarshal must use the same names.
func collidingNamedFields() idl.IdlDefinedFieldsNamed {
	return idl.IdlDefinedFieldsNamed{
		{Name: "foo_bar", Ty: &idltype.U8{}},
		{Name: "fooBar", Ty: &idltype.U8{}},
	}
}

func TestGenerateUniqueFieldNames_collidingIDLNames(t *testing.T) {
	fields := collidingNamedFields()
	m := generateUniqueFieldNames(fields)
	require.Len(t, m, 2)
	assert.Equal(t, "FooBar", m["foo_bar"])
	assert.Equal(t, "FooBar1", m["fooBar"])
}

// TestMarshalUnmarshalCodegen_matchesUniqueStructFieldNames documents the regression where
// gen_MarshalWithEncoder_struct / gen_UnmarshalWithDecoder_struct used tools.ToCamelUpper(field.Name)
// for accessors instead of generateUniqueFieldNames: both fields targeted obj.FooBar, so one
// value was serialized twice and the FooBar1 sibling was never written or read.
//
// The expected assertions describe the correct fixed behavior; they fail until uniquified names
// are threaded through marshal/unmarshal generation.
func TestMarshalUnmarshalCodegen_matchesUniqueStructFieldNames(t *testing.T) {
	idlMinimal := &idl.Idl{}
	fields := collidingNamedFields()
	receiver := "CollideAccount"

	marshalCode := gen_MarshalWithEncoder_struct(
		idlMinimal,
		false,
		receiver,
		"",
		fields,
		true,
	)
	unmarshalCode := gen_UnmarshalWithDecoder_struct(
		idlMinimal,
		false,
		receiver,
		"",
		fields,
	)

	f := jen.NewFile("fixture")
	f.Add(marshalCode)
	f.Add(unmarshalCode)
	src := f.GoString()

	// Correct codegen must reference both uniquified struct fields.
	assert.Contains(t, src, "obj.FooBar1", "marshal/unmarshal must access the deconflicted FooBar1 field")

	// Buggy codegen encodes/decodes the same field twice; reject duplicate bare obj.FooBar
	// Encode/Decode when a second distinct IDL field exists.
	encodeFooBar := strings.Count(src, "Encode(obj.FooBar)")
	decodeFooBar := strings.Count(src, "Decode(&obj.FooBar)")
	assert.Equal(t, 1, encodeFooBar, "each IDL field must map to a single Encode(obj.<Field>); duplicate Encode(obj.FooBar) indicates silent corruption")
	assert.Equal(t, 1, decodeFooBar, "each IDL field must map to a single Decode(&obj.<Field>); duplicate Decode(&obj.FooBar) indicates silent corruption")

	assert.Contains(t, src, "Encode(obj.FooBar1)")
	assert.Contains(t, src, "Decode(&obj.FooBar1)")
}

func TestGenerateUniqueParamNames_collidingIDLNames(t *testing.T) {
	fields := collidingNamedFields()
	m := generateUniqueParamNames(fields)
	require.Len(t, m, 2)
	assert.NotEqual(t, m[fields[0].Name], m[fields[1].Name])
	b0 := formatParamName(fields[0].Name)
	b1 := formatParamName(fields[1].Name)
	if b0 == b1 {
		assert.Equal(t, b0+"1", m[fields[1].Name])
	}
}

func TestGenInstructionType_uniquifiesArgFieldsAndDecode(t *testing.T) {
	ins := idl.IdlInstruction{
		Name:     "do_test",
		Args:     []idl.IdlField(collidingNamedFields()),
		Accounts: []idl.IdlInstructionAccountItem{},
	}
	g := &Generator{idl: &idl.Idl{}, options: &GeneratorOptions{Package: "test"}}
	code, err := g.gen_instructionType(ins)
	require.NoError(t, err)

	f := jen.NewFile("test")
	f.Add(code)
	src := f.GoString()

	assert.Contains(t, src, "FooBar1")
	assert.Equal(t, 1, strings.Count(src, "Decode(&obj.FooBar)"))
	assert.Contains(t, src, "Decode(&obj.FooBar1)")
}

func TestGenInstructions_builderEncodesEachArgOnce(t *testing.T) {
	fields := collidingNamedFields()
	paramNames := generateUniqueParamNames(fields)

	idlData := &idl.Idl{
		Instructions: []idl.IdlInstruction{
			{
				Name:     "do_test",
				Args:     []idl.IdlField(fields),
				Accounts: []idl.IdlInstructionAccountItem{},
			},
		},
	}
	gen := &Generator{idl: idlData, options: &GeneratorOptions{Package: "test"}}
	out, err := gen.gen_instructions()
	require.NoError(t, err)
	s := out.File.GoString()

	p0 := paramNames[fields[0].Name]
	p1 := paramNames[fields[1].Name]
	assert.Equal(t, 1, strings.Count(s, "Encode("+p0+")"), "each arg must be encoded exactly once")
	assert.Equal(t, 1, strings.Count(s, "Encode("+p1+")"))
	assert.NotEqual(t, p0, p1)
}
