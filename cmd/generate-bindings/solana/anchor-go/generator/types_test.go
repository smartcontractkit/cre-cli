//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"testing"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeComplexEnumIDL(enumName string) *idl.Idl {
	enumType := &idl.IdlTypeDefTyEnum{
		Kind: "enum",
		Variants: idl.VariantSlice{
			{Name: "Simple"},
			{
				Name: "WithFields",
				Fields: idl.Some[idl.IdlDefinedFields](idl.IdlDefinedFieldsNamed{
					{Name: "value", Ty: &idltype.U64{}},
				}),
			},
		},
	}

	return &idl.Idl{
		Types: idl.IdTypeDef_slice{
			{
				Name: enumName,
				Ty:   enumType,
			},
		},
	}
}

func TestGenComplexEnum_ConsecutiveUppercase(t *testing.T) {
	// "HTTPStatus" is stored in the IDL as-is. ToCamelUpper converts it to
	// "HttpStatus" (via snake_case intermediary), so ByName("HttpStatus")
	// won't find the original "HTTPStatus" entry and returns nil.
	idlData := makeComplexEnumIDL("HTTPStatus")
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	// Register the complex enum as the generator normally would.
	for _, typ := range gen.idl.Types {
		registerComplexEnums(typ)
	}

	outputFile, err := gen.genfile_types()
	require.NoError(t, err, "genfile_types should not panic or error for enum named HTTPStatus")
	require.NotNil(t, outputFile)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "HttpStatus")
}

func TestGenComplexEnum_SnakeCaseName(t *testing.T) {
	// "my_status" is stored in the IDL. ToCamelUpper converts it to
	// "MyStatus", so ByName("MyStatus") won't find "my_status".
	idlData := makeComplexEnumIDL("my_status")
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	for _, typ := range gen.idl.Types {
		registerComplexEnums(typ)
	}

	outputFile, err := gen.genfile_types()
	require.NoError(t, err, "genfile_types should not panic or error for enum named my_status")
	require.NotNil(t, outputFile)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "MyStatus")
}

func TestGenComplexEnum_AlreadyCamelCase(t *testing.T) {
	// "MyStatus" is already CamelCase. ToCamelUpper("MyStatus") == "MyStatus",
	// so ByName should find it. This should always work.
	idlData := makeComplexEnumIDL("MyStatus")
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	for _, typ := range gen.idl.Types {
		registerComplexEnums(typ)
	}

	outputFile, err := gen.genfile_types()
	require.NoError(t, err, "genfile_types should not panic or error for enum named MyStatus")
	require.NotNil(t, outputFile)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "MyStatus")
}
