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

// complexEnumIDL returns a minimal IDL containing a two-variant complex enum
// ("MyAction") suitable for exercising gen_complexEnum codegen.
func complexEnumIDL() *idl.Idl {
	enumType := &idl.IdlTypeDefTyEnum{
		Variants: idl.VariantSlice{
			{
				Name: "Transfer",
				Fields: idl.Some[idl.IdlDefinedFields](idl.IdlDefinedFieldsNamed{
					{Name: "amount", Ty: &idltype.U64{}},
				}),
			},
			{
				Name: "Burn",
				Fields: idl.Some[idl.IdlDefinedFields](idl.IdlDefinedFieldsNamed{
					{Name: "quantity", Ty: &idltype.U32{}},
				}),
			},
		},
	}
	return &idl.Idl{
		Types: idl.IdTypeDef_slice{
			{
				Name: "MyAction",
				Ty:   enumType,
			},
		},
	}
}

func genComplexEnumSource(t *testing.T) string {
	t.Helper()
	idlData := complexEnumIDL()
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	enumType := idlData.Types[0].Ty.(*idl.IdlTypeDefTyEnum)
	code, err := gen.gen_complexEnum("MyAction", nil, *enumType)
	require.NoError(t, err)

	f := jen.NewFile("test")
	f.Add(code)
	return f.GoString()
}

func TestComplexEnumEncode_nilInterfaceReturnsError(t *testing.T) {
	src := genComplexEnumSource(t)

	assert.Contains(t, src, "case nil:", "encoder must reject nil interface values")
	assert.Contains(t, src, `cannot encode nil value`, "nil case must return a descriptive error")
}

func TestComplexEnumEncode_defaultArmReturnsError(t *testing.T) {
	src := genComplexEnumSource(t)

	assert.Contains(t, src, "default:", "encoder must reject unknown variant types")
	assert.Contains(t, src, `unknown variant type`, "default case must return a descriptive error")
}

func TestComplexEnumEncode_nilPointerGuardPerVariant(t *testing.T) {
	src := genComplexEnumSource(t)

	assert.Contains(t, src, "realvalue == nil", "each variant case must guard against typed nil pointers")
	assert.Contains(t, src, `cannot encode nil *MyAction_Transfer`,
		"Transfer variant must have a nil-pointer error message")
	assert.Contains(t, src, `cannot encode nil *MyAction_Burn`,
		"Burn variant must have a nil-pointer error message")

	nilGuards := strings.Count(src, "realvalue == nil")
	assert.Equal(t, 2, nilGuards, "must have exactly one nil-pointer guard per variant")
}
