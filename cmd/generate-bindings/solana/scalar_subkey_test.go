package solana_test

import (
	"fmt"
	"math"
	"testing"

	bin "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/require"

	bindings "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana/bindings"
)

// TestPrepareSubkeyValueFilterableScalars verifies every scalar type the Solana
// bindings generator can emit in <Event>Filters structs.
func TestPrepareSubkeyValueFilterableScalars(t *testing.T) {
	t.Parallel()

	priv, err := solanago.NewRandomPrivateKey()
	require.NoError(t, err)
	pubkey := priv.PublicKey()
	metadata := []byte{0xde, 0xad}
	u256 := [32]byte{1, 2, 3}

	tests := []struct {
		name  string
		value any
	}{
		{name: "u8", value: uint8(7)},
		{name: "i8", value: int8(-3)},
		{name: "u16", value: uint16(42)},
		{name: "i16", value: int16(-42)},
		{name: "u32", value: uint32(100)},
		{name: "i32", value: int32(-100)},
		{name: "f32", value: float32(1.25)},
		{name: "u64", value: uint64(1_000)},
		{name: "i64", value: int64(-1_000)},
		{name: "f64", value: float64(3.5)},
		{name: "string", value: "filter-me"},
		{name: "bytes", value: metadata},
		{name: "pubkey", value: pubkey},
		{name: "u256", value: u256},
		{name: "i256", value: u256},
		{name: "option_u64", value: uint64(9)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := bindings.PrepareSubkeyValue(tt.value)
			require.NoError(t, err)
			require.NotEmpty(t, got)

			encoded, err := bindings.EncodeIndexedValue(tt.value)
			require.NoError(t, err)
			require.Equal(t, got, encoded)
		})
	}
}

func TestPrepareSubkeyValueUnsupportedScalars(t *testing.T) {
	t.Parallel()

	unsupported := []any{
		true,
		bin.Uint128{Lo: 1, Hi: 0},
		bin.Int128{Lo: 1, Hi: 0},
	}
	for _, value := range unsupported {
		t.Run(fmtType(value), func(t *testing.T) {
			t.Parallel()
			_, err := bindings.PrepareSubkeyValue(value)
			require.Error(t, err)
		})
	}
}

func TestPrepareSubkeyValueInt64Encoding(t *testing.T) {
	got, err := bindings.PrepareSubkeyValue(int64(-1))
	require.NoError(t, err)
	require.Len(t, got, 8)
	require.Equal(t, byte(0xff), got[0])
}

func TestPrepareSubkeyValueFloatEncoding(t *testing.T) {
	got, err := bindings.PrepareSubkeyValue(float64(math.SmallestNonzeroFloat64))
	require.NoError(t, err)
	require.Len(t, got, 8)
}

func fmtType(v any) string {
	return fmt.Sprintf("%T", v)
}
