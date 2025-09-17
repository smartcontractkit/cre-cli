package logger

import (
	"encoding/hex"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

type EventDataWrapper struct {
	EventData map[string]interface{}
}

func (w EventDataWrapper) MarshalZerologObject(e *zerolog.Event) {
	for key, value := range w.EventData {
		switch v := value.(type) {
		case [32]byte:
			e.Str(key, hex.EncodeToString(v[:]))
		default:
			e.Interface(key, value)
		}
	}
}

type DecodedTransactionLogWrapper struct {
	seth.DecodedTransactionLog
}

func (dtw DecodedTransactionLogWrapper) MarshalZerologObject(e *zerolog.Event) {
	// Log fields of DecodedTransactionLog
	e.Uint64("BlockNumber", dtw.BlockNumber)
	e.Uint("Index", dtw.Index)
	e.Str("TxHash", dtw.TXHash)
	e.Uint("TxIndex", dtw.TXIndex)
	e.Bool("Removed", dtw.Removed)
	e.Str("FileTag", dtw.FileTag)

	// Log fields of embedded DecodedCommonLog
	e.Str("Signature", dtw.Signature)
	e.Str("Address", dtw.Address.Hex())
	e.Strs("Topics", dtw.Topics)

	// Handle EventData with EventDataWrapper
	if dtw.EventData != nil {
		e.Object("EventData", EventDataWrapper{EventData: dtw.EventData})
	}
}
