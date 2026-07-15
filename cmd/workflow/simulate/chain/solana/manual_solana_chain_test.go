package solana

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
)

func mustPubkey(t *testing.T) solana.PublicKey {
	t.Helper()
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	return pk.PublicKey()
}

func TestManualSolanaChain_ManualTrigger_Delivers(t *testing.T) {
	t.Parallel()

	m := NewManualSolanaChain(nil) // inner unused on the trigger path
	prog := mustPubkey(t)
	const triggerID = "solana:ChainSelector:16423721717087811551@1.0.0"

	ch, cerr := m.RegisterLogTrigger(context.Background(), triggerID, commonCap.RequestMetadata{}, &solcap.FilterLogTriggerRequest{
		Address: prog.Bytes(),
	})
	require.Nil(t, cerr)

	log := &solcap.Log{Address: prog.Bytes(), Data: []byte("event-body"), LogIndex: 0}
	require.NoError(t, m.ManualTrigger(context.Background(), triggerID, log))

	select {
	case got := <-ch:
		assert.Equal(t, prog.Bytes(), got.Trigger.GetAddress())
		assert.NotEmpty(t, got.Id)
	case <-time.After(time.Second):
		t.Fatal("expected trigger event, got none")
	}
}

func TestManualSolanaChain_ManualTrigger_FilterRejectsWrongAddress(t *testing.T) {
	t.Parallel()

	m := NewManualSolanaChain(nil)
	const triggerID = "t1"

	_, cerr := m.RegisterLogTrigger(context.Background(), triggerID, commonCap.RequestMetadata{}, &solcap.FilterLogTriggerRequest{
		Address: mustPubkey(t).Bytes(),
	})
	require.Nil(t, cerr)

	// Different program address -> filter must reject.
	log := &solcap.Log{Address: mustPubkey(t).Bytes()}
	err := m.ManualTrigger(context.Background(), triggerID, log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match filter address")
}

func TestManualSolanaChain_ManualTrigger_Unregistered(t *testing.T) {
	t.Parallel()

	m := NewManualSolanaChain(nil)
	err := m.ManualTrigger(context.Background(), "nope", &solcap.Log{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not registered")
}

func TestParseAnchorEvents_AttributesEmittingProgram(t *testing.T) {
	t.Parallel()

	prog := mustPubkey(t)
	inner := mustPubkey(t)
	ev0 := append(make([]byte, 0, 16), []byte{1, 2, 3, 4, 5, 6, 7, 8}...)
	ev0 = append(ev0, []byte("body0")...)
	ev1 := append(make([]byte, 0, 16), []byte{9, 9, 9, 9, 9, 9, 9, 9}...)
	ev1 = append(ev1, []byte("body1")...)

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

func TestGetSolanaTriggerLogFromValues_Validation(t *testing.T) {
	t.Parallel()

	_, err := GetSolanaTriggerLogFromValues(context.Background(), nil, "  ", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature cannot be empty")

	_, err = GetSolanaTriggerLogFromValues(context.Background(), nil, "not-base58!!", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction signature")
}
