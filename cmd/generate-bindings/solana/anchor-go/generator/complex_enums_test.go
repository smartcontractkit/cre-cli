//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"strings"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/stretchr/testify/assert"
)

// complexEnumGuard mirrors the condition used in gen_marshal_DefinedFieldsNamed
// and gen_unmarshal_DefinedFieldsNamed to decide whether a field is routed to
// the specialized enum encoder/parser or falls through to the generic
// Encode/Decode path.
func complexEnumGuard(ty idltype.IdlType) bool {
	return isComplexEnum(ty) ||
		(IsArray(ty) && isComplexEnum(ty.(*idltype.Array).Type)) ||
		(IsVec(ty) && isComplexEnum(ty.(*idltype.Vec).Vec)) ||
		isOptionalComplexEnum(ty)
}

func TestComplexEnumGuard_handlesOptionAndCOption(t *testing.T) {
	const name = "Outcome"
	register_TypeName_as_ComplexEnum(name)
	t.Cleanup(func() { delete(typeRegistryComplexEnum, name) })

	defined := &idltype.Defined{Name: name}

	assert.True(t, complexEnumGuard(defined), "bare Defined")
	assert.True(t, complexEnumGuard(&idltype.Option{Option: defined}), "Option<ComplexEnum>")
	assert.True(t, complexEnumGuard(&idltype.COption{COption: defined}), "COption<ComplexEnum>")
}

// TestComplexEnumGuard_rejectsNonComplexOptionals ensures the guard does NOT
// fire for Option/COption wrapping a non-complex Defined or a primitive.
// A false positive here would cause the switch to enter the Option/COption case
// where .Option.(*idltype.Defined) would panic on a non-Defined inner type.
func TestComplexEnumGuard_rejectsNonComplexOptionals(t *testing.T) {
	const complexName = "Outcome"
	register_TypeName_as_ComplexEnum(complexName)
	t.Cleanup(func() { delete(typeRegistryComplexEnum, complexName) })

	nonComplex := &idltype.Defined{Name: "PlainStruct"}

	assert.False(t, complexEnumGuard(&idltype.Option{Option: nonComplex}),
		"Option<NonComplexDefined> must not trigger the complex-enum path")
	assert.False(t, complexEnumGuard(&idltype.COption{COption: nonComplex}),
		"COption<NonComplexDefined> must not trigger the complex-enum path")
	assert.False(t, complexEnumGuard(&idltype.Option{Option: &idltype.U64{}}),
		"Option<U64> must not trigger the complex-enum path")
	assert.False(t, complexEnumGuard(&idltype.COption{COption: &idltype.U8{}}),
		"COption<U8> must not trigger the complex-enum path")
	assert.False(t, complexEnumGuard(&idltype.Option{Option: &idltype.Vec{Vec: &idltype.Defined{Name: complexName}}}),
		"Option<Vec<ComplexEnum>> — nested containers not supported, must not match")
}

// TestComplexEnumCodegen_optionalComplexEnum runs the actual marshal/unmarshal
// generator with Option<ComplexEnum> and COption<ComplexEnum> fields and
// verifies the generated Go source uses the specialized enum encoder/parser
// instead of the generic Encode/Decode.
func TestComplexEnumCodegen_optionalComplexEnum(t *testing.T) {
	const enumName = "Outcome"
	register_TypeName_as_ComplexEnum(enumName)
	t.Cleanup(func() { delete(typeRegistryComplexEnum, enumName) })

	fields := idl.IdlDefinedFieldsNamed{
		{Name: "id", Ty: &idltype.U64{}},
		{Name: "verdict", Ty: &idltype.Option{Option: &idltype.Defined{Name: enumName}}},
		{Name: "alt_verdict", Ty: &idltype.COption{COption: &idltype.Defined{Name: enumName}}},
		{Name: "checksum", Ty: &idltype.U64{}},
	}

	marshalCode := gen_MarshalWithEncoder_struct(
		&idl.Idl{}, false, "Report", "", fields, true,
	)
	unmarshalCode := gen_UnmarshalWithDecoder_struct(
		&idl.Idl{}, false, "Report", "", fields,
	)

	f := jen.NewFile("fixture")
	f.Add(marshalCode)
	f.Add(unmarshalCode)
	src := f.GoString()

	// Specialized enum encoder/parser must appear.
	assert.Contains(t, src, "EncodeOutcome",
		"Option/COption<ComplexEnum> fields must call the specialized enum encoder")
	assert.Contains(t, src, "DecodeOutcome",
		"Option/COption<ComplexEnum> fields must call the specialized enum parser")

	// Option flags must still be written/read.
	assert.Contains(t, src, "WriteOption")
	assert.Contains(t, src, "WriteCOption")
	assert.Contains(t, src, "ReadOption")
	assert.Contains(t, src, "ReadCOption")

	// Only the two plain U64 fields (Id, Checksum) should use the generic
	// encoder/decoder. If the enum fields also fall through, the count is 4.
	assert.Equal(t, 2, strings.Count(src, ".Encode("),
		"generic Encode must only be used for non-enum fields")
	assert.Equal(t, 2, strings.Count(src, ".Decode("),
		"generic Decode must only be used for non-enum fields")
}
