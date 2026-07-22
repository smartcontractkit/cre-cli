package solana

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	logpollertypes "github.com/smartcontractkit/chainlink-solana/pkg/solana/logpoller/types"
)

const testAnchorEventLogPrefix = "Program data: "

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
		testAnchorEventLogPrefix + base64.StdEncoding.EncodeToString(ev0),
		"Program " + inner.String() + " invoke [2]",
		testAnchorEventLogPrefix + base64.StdEncoding.EncodeToString(ev1),
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

	_, err := GetSolanaTriggerLogWithFilter(context.Background(), nil, "  ", 0, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature cannot be empty")

	_, err = GetSolanaTriggerLogWithFilter(context.Background(), nil, "not-base58!!", 0, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction signature")
}

func TestExtractSolanaCPIEvents_AnchorEvent(t *testing.T) {
	t.Parallel()

	source := mustPubkey(t)
	dest := mustPubkey(t)
	other := mustPubkey(t)
	eventData := append([]byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte("payload")...)
	methodSig := logpollertypes.AnchorCPIEventDiscriminator()
	instructionData := append(methodSig[:], eventData...)

	tx := &solana.Transaction{
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{source, dest, other},
			Instructions: []solana.CompiledInstruction{
				{ProgramIDIndex: 0},
			},
		},
	}
	meta := &solanarpc.TransactionMeta{
		InnerInstructions: []solanarpc.InnerInstruction{
			{
				Index: 0,
				Instructions: []solanarpc.CompiledInstruction{
					{ProgramIDIndex: 2, Data: solana.Base58(instructionData), StackHeight: 2},
					{ProgramIDIndex: 1, Data: solana.Base58(instructionData), StackHeight: 2},
				},
			},
		},
	}
	filter := &solcap.FilterLogTriggerRequest{
		Address: source.Bytes(),
		CpiFilterConfig: &solcap.CPIFilterConfig{
			DestAddress: dest.Bytes(),
			MethodName:  []byte(logpollertypes.AnchorCPIMethodName),
		},
	}

	events, err := extractSolanaCPIEvents(tx, meta, filter)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, source, events[0].programID)
	assert.Equal(t, eventData, events[0].data)
}

func TestExtractSolanaCPIEvents_Validation(t *testing.T) {
	t.Parallel()

	source := mustPubkey(t)
	dest := mustPubkey(t)
	tx := &solana.Transaction{}
	meta := &solanarpc.TransactionMeta{}

	_, err := extractSolanaCPIEvents(tx, meta, &solcap.FilterLogTriggerRequest{
		Address: []byte{0x01},
		CpiFilterConfig: &solcap.CPIFilterConfig{
			DestAddress: dest.Bytes(),
			MethodName:  []byte(logpollertypes.AnchorCPIMethodName),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source address must be 32 bytes")

	_, err = extractSolanaCPIEvents(tx, meta, &solcap.FilterLogTriggerRequest{
		Address: source.Bytes(),
		CpiFilterConfig: &solcap.CPIFilterConfig{
			DestAddress: []byte{0x01},
			MethodName:  []byte(logpollertypes.AnchorCPIMethodName),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "destination address must be 32 bytes")

	_, err = extractSolanaCPIEvents(tx, meta, &solcap.FilterLogTriggerRequest{
		Address: source.Bytes(),
		CpiFilterConfig: &solcap.CPIFilterConfig{
			DestAddress: dest.Bytes(),
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method name cannot be empty")
}

func TestDecodeLogTriggerConfig(t *testing.T) {
	t.Parallel()

	_, err := decodeLogTriggerConfig(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no payload")

	wrong, err := anypb.New(durationpb.New(0))
	require.NoError(t, err)
	_, err = decodeLogTriggerConfig(wrong)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FilterLogTriggerRequest")

	msg, err := anypb.New(&solcap.FilterLogTriggerRequest{EventName: "MessageEmitted"})
	require.NoError(t, err)
	cfg, err := decodeLogTriggerConfig(msg)
	require.NoError(t, err)
	assert.Equal(t, "MessageEmitted", cfg.GetEventName())
}
