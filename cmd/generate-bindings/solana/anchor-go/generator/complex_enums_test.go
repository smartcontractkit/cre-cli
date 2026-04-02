//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"testing"

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

// TestComplexEnumGuard_handlesOptionAndCOption verifies that the guard used in
// gen_marshal_DefinedFieldsNamed / gen_unmarshal_DefinedFieldsNamed correctly
// recognises Option<ComplexEnum> and COption<ComplexEnum> so that they are
// routed to the specialized enum encoder/parser instead of the generic
// Encode/Decode path.
func TestComplexEnumGuard_handlesOptionAndCOption(t *testing.T) {
	const name = "Outcome"
	register_TypeName_as_ComplexEnum(name)
	t.Cleanup(func() { delete(typeRegistryComplexEnum, name) })

	defined := &idltype.Defined{Name: name}

	assert.True(t, complexEnumGuard(defined), "bare Defined: already handled")
	assert.True(t, complexEnumGuard(&idltype.Option{Option: defined}),
		"Option<ComplexEnum> falls through to generic Encode/Decode, omitting the enum discriminant")
	assert.True(t, complexEnumGuard(&idltype.COption{COption: defined}),
		"COption<ComplexEnum> falls through to generic Encode/Decode, omitting the enum discriminant")
}
