package generator

import (
	"testing"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func testProgramID(t *testing.T) *solana.PublicKey {
	t.Helper()
	pk, err := solana.PublicKeyFromBase58("ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL")
	require.NoError(t, err)
	return &pk
}

func minimalInstruction(name string) idl.IdlInstruction {
	return idl.IdlInstruction{
		Name:          name,
		Discriminator: idl.IdlDiscriminator{175, 175, 109, 31, 13, 152, 155, 237},
		Accounts:      []idl.IdlInstructionAccountItem{},
		Args:          []idl.IdlField{},
	}
}

func TestValidateIDLDerivedIdentifiers_valid(t *testing.T) {
	i := &idl.Idl{
		Address:      testProgramID(t),
		Instructions: []idl.IdlInstruction{minimalInstruction("initialize")},
	}
	require.NoError(t, ValidateIDLDerivedIdentifiers(i))
}

func TestValidateIDLDerivedIdentifiers_invalidConstantName(t *testing.T) {
	i := &idl.Idl{
		Address:      testProgramID(t),
		Instructions: []idl.IdlInstruction{minimalInstruction("initialize")},
		Constants: []idl.IdlConst{{
			Name:  "123bad",
			Ty:    &idltype.U8{},
			Value: "1",
		}},
	}
	err := ValidateIDLDerivedIdentifiers(i)
	require.Error(t, err)
	require.Contains(t, err.Error(), "123bad")
}
