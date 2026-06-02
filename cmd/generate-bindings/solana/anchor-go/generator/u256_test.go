//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"testing"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDLTypeKind_ToTypeDeclCode_U256(t *testing.T) {
	assert.NotPanics(t, func() {
		result := IDLTypeKind_ToTypeDeclCode(&idltype.U256{})
		assert.NotNil(t, result)
	})
}

func TestIDLTypeKind_ToTypeDeclCode_I256(t *testing.T) {
	assert.NotPanics(t, func() {
		result := IDLTypeKind_ToTypeDeclCode(&idltype.I256{})
		assert.NotNil(t, result)
	})
}

func TestGenTypeName_U256(t *testing.T) {
	assert.NotPanics(t, func() {
		result := genTypeName(&idltype.U256{})
		assert.NotNil(t, result)
	})
}

func TestGenTypeName_I256(t *testing.T) {
	assert.NotPanics(t, func() {
		result := genTypeName(&idltype.I256{})
		assert.NotNil(t, result)
	})
}

func TestGenConstants_U256(t *testing.T) {
	idlData := &idl.Idl{
		Constants: []idl.IdlConst{
			{
				Name:  "MAX_SUPPLY",
				Ty:    &idltype.U256{},
				Value: "115792089237316195423570985008687907853269984665640564039457584007913129639935",
			},
		},
	}
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	outputFile, err := gen.gen_constants()
	require.NoError(t, err)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "var MAX_SUPPLY = func() *big.Int")
	assert.Contains(t, generatedCode, ".SetString(\"115792089237316195423570985008687907853269984665640564039457584007913129639935\", 10)")
}

func TestGenConstants_I256(t *testing.T) {
	idlData := &idl.Idl{
		Constants: []idl.IdlConst{
			{
				Name:  "MIN_VALUE",
				Ty:    &idltype.I256{},
				Value: "-57896044618658097711785492504343953926634992332820282019728792003956564819968",
			},
		},
	}
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	outputFile, err := gen.gen_constants()
	require.NoError(t, err)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "var MIN_VALUE = func() *big.Int")
	assert.Contains(t, generatedCode, ".SetString(\"-57896044618658097711785492504343953926634992332820282019728792003956564819968\", 10)")
}

func TestGenConstants_U256_Invalid(t *testing.T) {
	idlData := &idl.Idl{
		Constants: []idl.IdlConst{
			{
				Name:  "INVALID_U256",
				Ty:    &idltype.U256{},
				Value: "not_a_number",
			},
		},
	}
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	_, err := gen.gen_constants()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse u256")
}

func TestGenConstants_I256_Invalid(t *testing.T) {
	idlData := &idl.Idl{
		Constants: []idl.IdlConst{
			{
				Name:  "INVALID_I256",
				Ty:    &idltype.I256{},
				Value: "not_a_number",
			},
		},
	}
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	_, err := gen.gen_constants()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse i256")
}

func TestGenConstants_U256_WithUnderscores(t *testing.T) {
	idlData := &idl.Idl{
		Constants: []idl.IdlConst{
			{
				Name:  "LARGE_U256",
				Ty:    &idltype.U256{},
				Value: "1_000_000_000_000_000_000_000_000_000",
			},
		},
	}
	gen := &Generator{
		idl:     idlData,
		options: &GeneratorOptions{Package: "test"},
	}

	outputFile, err := gen.gen_constants()
	require.NoError(t, err)

	generatedCode := outputFile.File.GoString()
	assert.Contains(t, generatedCode, "var LARGE_U256 = func() *big.Int")
	assert.Contains(t, generatedCode, ".SetString(\"1000000000000000000000000000\", 10)")
}
