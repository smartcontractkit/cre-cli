package solana

import (
	"time"

	"github.com/smartcontractkit/chainlink-common/pkg/types/query/primitives"
	pb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	solanatypes "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/types"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
)

type FilterLogTriggerRequest struct {
	Address       solanatypes.PublicKey
	EventName     string
	EventSig      solanatypes.EventSignature
	EventIdl      solanatypes.EventIdl
	SubkeyPaths   solanatypes.SubKeyPaths
	SubkeyFilters []SubkeyFilterCriteria
}
type SubkeyFilterCriteria struct {
	SubkeyIndex uint64
	Comparers   []primitives.ValueComparator
}

type SubKeyPathAndFilter struct {
	SubkeyPath string
	Value      any
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

type Log struct {
	ID             int64
	FilterID       int64
	ChainID        string
	LogIndex       int64
	BlockHash      solanatypes.Hash
	BlockNumber    int64
	BlockTimestamp time.Time
	Address        solanatypes.PublicKey
	EventSig       solanatypes.EventSignature
	SubkeyValues   solanatypes.IndexedValues
	TxHash         solanatypes.Signature
	Data           []byte
	CreatedAt      time.Time
	ExpiresAt      *time.Time
	SequenceNum    int64
	Error          *string
}

func (*Log) ProtoMessage() {}

func (x *Log) ProtoReflect() protoreflect.Message {
	return nil // not implemented
}
