//nolint:all // Forked from anchor-go generator, maintaining original code structure
package generator

import (
	"testing"

	"github.com/gagliardetto/anchor-go/idl/idltype"
	"github.com/stretchr/testify/require"
)

func TestIsFilterableField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ty   idltype.IdlType
		want bool
	}{
		{name: "pubkey", ty: &idltype.Pubkey{}, want: true},
		{name: "string", ty: &idltype.String{}, want: true},
		{name: "bytes", ty: &idltype.Bytes{}, want: true},
		{name: "u64", ty: &idltype.U64{}, want: true},
		{name: "option_u64", ty: &idltype.Option{Option: &idltype.U64{}}, want: true},
		{name: "defined", ty: &idltype.Defined{Name: "UserData"}, want: false},
		{name: "vec", ty: &idltype.Vec{Vec: &idltype.U8{}}, want: false},
		{name: "array", ty: &idltype.Array{Type: &idltype.U8{}, Size: &idltype.IdlArrayLenValue{Value: 4}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, isFilterableField(tt.ty))
		})
	}
}
