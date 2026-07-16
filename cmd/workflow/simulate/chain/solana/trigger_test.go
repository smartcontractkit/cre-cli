package solana

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustPubkey(t *testing.T) solana.PublicKey {
	t.Helper()
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	return pk.PublicKey()
}

func TestParseAnchorEvents_AttributesEmittingProgram(t *testing.T) {
	t.Parallel()

	prog := mustPubkey(t)
	inner := mustPubkey(t)
	ev0 := append([]byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte("body0")...)
	ev1 := append([]byte{9, 9, 9, 9, 9, 9, 9, 9}, []byte("body1")...)

	logs := []string{
		"Program " + prog.String() + " invoke [1]",
		"Program log: Instruction: DoThing",
		anchorEventLogPrefix + base64.StdEncoding.EncodeToString(ev0),
		"Program " + inner.String() + " invoke [2]",
		anchorEventLogPrefix + base64.StdEncoding.EncodeToString(ev1),
		"Program " + inner.String() + " success",
		"Program " + prog.String() + " success",
	}

	events, err := parseAnchorEvents(logs)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, prog, events[0].programID)
	assert.Equal(t, ev0, events[0].data)
	assert.Equal(t, inner, events[1].programID)
	assert.Equal(t, ev1, events[1].data)
}

func TestParseAnchorEvents_NoEvents(t *testing.T) {
	t.Parallel()

	prog := mustPubkey(t)
	logs := []string{
		"Program " + prog.String() + " invoke [1]",
		"Program log: Instruction: DoThing",
		"Program " + prog.String() + " success",
	}
	events, err := parseAnchorEvents(logs)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestGetSolanaTriggerLogFromValues_Validation(t *testing.T) {
	t.Parallel()

	_, err := GetSolanaTriggerLogFromValues(context.Background(), nil, "  ", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature cannot be empty")

	_, err = GetSolanaTriggerLogFromValues(context.Background(), nil, "not-base58!!", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction signature")
}
