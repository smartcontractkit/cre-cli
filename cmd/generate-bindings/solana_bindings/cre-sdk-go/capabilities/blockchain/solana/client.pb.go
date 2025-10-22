package solana

import (
	"time"

	pb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	solanatypes "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/types"
	"google.golang.org/protobuf/runtime/protoimpl"
)

type FilterLogTriggerRequest struct {
	ID              int64 // only for internal usage. Values set externally are ignored.
	Name            string
	Address         solanatypes.PublicKey
	EventName       string
	EventSig        solanatypes.EventSignature
	StartingBlock   int64
	EventIdl        solanatypes.EventIdl
	SubkeyPaths     solanatypes.SubKeyPaths
	Retention       time.Duration
	MaxLogsKept     int64
	IsDeleted       bool // only for internal usage. Values set externally are ignored.
	IsBackfilled    bool // only for internal usage. Values set externally are ignored.
	IncludeReverted bool
}

type ReadAccountRequest struct {
	Call        *ReadAccountMsg
	BlockNumber *pb.BigInt
}

type ReadAccountMsg struct {
	AccountAddress solanatypes.PublicKey
}

type ReadAccountReply struct {
	Data []byte
}

type WriteReportReply struct {
	state protoimpl.MessageState `protogen:"open.v1"`
}
