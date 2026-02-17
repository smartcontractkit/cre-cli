package solana

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gagliardetto/anchor-go/errors"
	binary "github.com/gagliardetto/binary"
	"github.com/smartcontractkit/chainlink-common/pkg/types/query/primitives"
	solanatypes "github.com/smartcontractkit/chainlink-solana/pkg/solana/logpoller/types"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana/cre-sdk-go/anchorcodec"

	// solanatypes "github.com/chainlink-solana/pkg/solana/logpoller/types"

	"github.com/smartcontractkit/cre-sdk-go/cre"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	PublicKeyLength = 32
	SignatureLength = 64
)

type FilterLogTriggerRequest struct {
	Address       []byte
	EventName     string
	EventSig      solanatypes.EventSignature
	EventIdl      anchorcodec.EventIDLTypes // this is the only change
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

func LogTrigger(chainSelector uint64, config *FilterLogTriggerRequest) cre.Trigger[*Log, *Log] {
	return nil
}

func (*Log) ProtoMessage() {}

func (x *Log) ProtoReflect() protoreflect.Message {
	return nil // not implemented
}

type ForwarderReport struct {
	AccountHash [32]byte `json:"account_hash"`
	Payload     []byte   `json:"payload"`
}

func (obj ForwarderReport) MarshalWithEncoder(encoder *binary.Encoder) (err error) {
	// Serialize `AccountHash`:
	err = encoder.Encode(obj.AccountHash)
	if err != nil {
		return errors.NewField("AccountHash", err)
	}
	// Serialize `Payload`:
	err = encoder.Encode(obj.Payload)
	if err != nil {
		return errors.NewField("Payload", err)
	}
	return nil
}

func (obj ForwarderReport) Marshal() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	encoder := binary.NewBorshEncoder(buf)
	err := obj.MarshalWithEncoder(encoder)
	if err != nil {
		return nil, fmt.Errorf("error while encoding ForwarderReport: %w", err)
	}
	return buf.Bytes(), nil
}

// type Client struct {
// 	ChainSelector uint64
// 	// TODO: https://smartcontract-it.atlassian.net/browse/CAPPL-799 allow defaults for capabilities
// }

// func (c *Client) GetAccountInfoWithOpts(runtime cre.Runtime, req *GetAccountInfoRequest) cre.Promise[*GetAccountInfoReply] {
// 	wrapped := &anypb.Any{}

// 	capCallResponse := cre.Then(runtime.CallCapability(&sdkpb.CapabilityRequest{
// 		Id:      "solana" + ":ChainSelector:" + strconv.FormatUint(c.ChainSelector, 10) + "@1.0.0",
// 		Payload: wrapped,
// 		Method:  "GetAccountInfoWithOpts",
// 	}), func(i *sdkpb.CapabilityResponse) (*GetAccountInfoReply, error) {
// 		switch payload := i.Response.(type) {
// 		case *sdkpb.CapabilityResponse_Error:
// 			return nil, errors.New(payload.Error)
// 		case *sdkpb.CapabilityResponse_Payload:
// 			output := &GetAccountInfoReply{}
// 			err := payload.Payload.UnmarshalTo(output)
// 			return output, err
// 		default:
// 			return nil, errors.New("unexpected response type")
// 		}
// 	})

// 	return capCallResponse
// }

// func (c *Client) GetMultipleAccountsWithOpts(runtime cre.Runtime, req GetMultipleAccountsRequest) cre.Promise[*GetMultipleAccountsReply] {
// 	return cre.PromiseFromResult[*GetMultipleAccountsReply](nil, nil)
// }

// func (c *Client) SimulateTX(runtime cre.Runtime, input *SimulateTXRequest) cre.Promise[*SimulateTXReply] {
// 	return cre.PromiseFromResult[*SimulateTXReply](nil, nil)
// }

// func (c *Client) WriteReport(runtime cre.Runtime, input *WriteCreReportRequest) cre.Promise[*WriteReportReply] {
// 	wrapped := &anypb.Any{}

// 	capCallResponse := cre.Then(runtime.CallCapability(&sdkpb.CapabilityRequest{
// 		Id:      "solana" + ":ChainSelector:" + strconv.FormatUint(c.ChainSelector, 10) + "@1.0.0",
// 		Payload: wrapped,
// 		Method:  "WriteReport",
// 	}), func(i *sdkpb.CapabilityResponse) (*WriteReportReply, error) {
// 		switch payload := i.Response.(type) {
// 		case *sdkpb.CapabilityResponse_Error:
// 			return nil, errors.New(payload.Error)
// 		case *sdkpb.CapabilityResponse_Payload:
// 			output := &WriteReportReply{}
// 			err := payload.Payload.UnmarshalTo(output)
// 			return output, err
// 		default:
// 			return nil, errors.New("unexpected response type")
// 		}
// 	})

// 	return capCallResponse
// }

// type SimulateTXRequest struct {
// 	Receiver           solanatypes.PublicKey
// 	EncodedTransaction []byte
// 	Opts               *SimulateTXOpts
// }

// type SimulateTXOpts struct {
// 	// If true the transaction signatures will be verified
// 	// (default: false, conflicts with ReplaceRecentBlockhash)
// 	SigVerify bool

// 	// Commitment level to simulate the transaction at.
// 	// (default: "finalized").
// 	Commitment CommitmentType

// 	// If true the transaction recent blockhash will be replaced with the most recent blockhash.
// 	// (default: false, conflicts with SigVerify)
// 	ReplaceRecentBlockhash bool

// 	Accounts *SimulateTransactionAccountsOpts
// }

// type SimulateTransactionAccountsOpts struct {
// 	// (optional) Encoding for returned Account data,
// 	// either "base64" (default), "base64+zstd" or "jsonParsed".
// 	// - "jsonParsed" encoding attempts to use program-specific state parsers
// 	//   to return more human-readable and explicit account state data.
// 	//   If "jsonParsed" is requested but a parser cannot be found,
// 	//   the field falls back to binary encoding, detectable when
// 	//   the data field is type <string>.
// 	Encoding EncodingType

// 	// An array of accounts to return.
// 	Addresses []solanatypes.PublicKey
// }

// type SimulateTXReply struct {
// 	// Error if transaction failed, null if transaction succeeded.
// 	Err *string

// 	// Array of log messages the transaction instructions output during execution,
// 	// null if simulation failed before the transaction was able to execute
// 	// (for example due to an invalid blockhash or signature verification failure)
// 	Logs []string
// 	// Array of accounts with the same length as the accounts.addresses array in the request.
// 	Accounts []*Account

// 	// The number of compute budget units consumed during the processing of this transaction.
// 	UnitsConsumed *uint64
// }

// represents solana-go EncodingType
// type EncodingType string

// const (
// 	EncodingBase58     EncodingType = "base58"      // limited to Account data of less than 129 bytes
// 	EncodingBase64     EncodingType = "base64"      // will return base64 encoded data for Account data of any size
// 	EncodingBase64Zstd EncodingType = "base64+zstd" // compresses the Account data using Zstandard and base64-encodes the result

// 	// attempts to use program-specific state parsers to
// 	// return more human-readable and explicit account state data.
// 	// If "jsonParsed" is requested but a parser cannot be found,
// 	// the field falls back to "base64" encoding, detectable when the data field is type <string>.
// 	// Cannot be used if specifying dataSlice parameters (offset, length).
// 	EncodingJSONParsed EncodingType = "jsonParsed"

// 	EncodingJSON EncodingType = "json" // NOTE: you're probably looking for EncodingJSONParsed
// )

// represents solana-go CommitmentType
// type CommitmentType string

// const (
// 	// The node will query the most recent block confirmed by supermajority
// 	// of the cluster as having reached maximum lockout,
// 	// meaning the cluster has recognized this block as finalized.
// 	CommitmentFinalized CommitmentType = "finalized"

// 	// The node will query the most recent block that has been voted on by supermajority of the cluster.
// 	// - It incorporates votes from gossip and replay.
// 	// - It does not count votes on descendants of a block, only direct votes on that block.
// 	// - This confirmation level also upholds "optimistic confirmation" guarantees in release 1.3 and onwards.
// 	CommitmentConfirmed CommitmentType = "confirmed"

// 	// The node will query its most recent block. Note that the block may still be skipped by the cluster.
// 	CommitmentProcessed CommitmentType = "processed"
// )

// type GetAccountInfoRequest struct {
// 	Account solanatypes.PublicKey
// 	Opts    *GetAccountInfoOpts
// }

// func (*GetAccountInfoRequest) ProtoMessage() {}

// func (x *GetAccountInfoRequest) ProtoReflect() protoreflect.Message {
// 	var file_capabilities_blockchain_evm_v1alpha_client_proto_msgTypes = make([]protoimpl.MessageInfo, 26)
// 	mi := &file_capabilities_blockchain_evm_v1alpha_client_proto_msgTypes[2]
// 	if x != nil {
// 		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
// 		if ms.LoadMessageInfo() == nil {
// 			ms.StoreMessageInfo(mi)
// 		}
// 		return ms
// 	}
// 	return mi.MessageOf(x)
// }

// type GetAccountInfoReply struct {
// 	RPCContext
// 	Value *Account
// }

// func (x *GetAccountInfoReply) ProtoReflect() protoreflect.Message {
// 	var file_capabilities_blockchain_evm_v1alpha_client_proto_msgTypes = make([]protoimpl.MessageInfo, 26)
// 	mi := &file_capabilities_blockchain_evm_v1alpha_client_proto_msgTypes[2]
// 	if x != nil {
// 		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
// 		if ms.LoadMessageInfo() == nil {
// 			ms.StoreMessageInfo(mi)
// 		}
// 		return ms
// 	}
// 	return mi.MessageOf(x)
// }

// type GetMultipleAccountsRequest struct {
// 	Accounts []solanatypes.PublicKey
// 	Opts     *GetMultipleAccountsOpts
// }

// type GetMultipleAccountsReply struct {
// 	RPCContext
// 	Value []*Account
// }

// represents solana-go PublicKey
// type PublicKey [PublicKeyLength]byte

// // represents solana-go Signature
// type Signature [SignatureLength]byte

// // represents solana-go Hash
// type Hash PublicKey

// represents solana-go AccountsMeta
// type AccountMeta struct {
// 	PublicKey  PublicKey
// 	IsWritable bool
// 	IsSigner   bool
// }

// represents solana-go AccountMetaSlice
// type AccountMetaSlice []*AccountMeta

// represents solana-go DataSlice
// type DataSlice struct {
// 	Offset *uint64
// 	Length *uint64
// }

// represents solana-go GetAccountInfoOpts
// type GetAccountInfoOpts struct {
// 	// Encoding for Account data.
// 	// Either "base58" (slow), "base64", "base64+zstd", or "jsonParsed".
// 	// - "base58" is limited to Account data of less than 129 bytes.
// 	// - "base64" will return base64 encoded data for Account data of any size.
// 	// - "base64+zstd" compresses the Account data using Zstandard and base64-encodes the result.
// 	// - "jsonParsed" encoding attempts to use program-specific state parsers to return more
// 	// 	 human-readable and explicit account state data. If "jsonParsed" is requested but a parser
// 	//   cannot be found, the field falls back to "base64" encoding,
// 	//   detectable when the data field is type <string>.
// 	//
// 	// This parameter is optional.
// 	Encoding EncodingType

// 	// Commitment requirement.
// 	//
// 	// This parameter is optional. Default value is Finalized
// 	Commitment CommitmentType

// 	// dataSlice parameters for limiting returned account data:
// 	// Limits the returned account data using the provided offset and length fields;
// 	// only available for "base58", "base64" or "base64+zstd" encodings.
// 	//
// 	// This parameter is optional.
// 	DataSlice *DataSlice

// 	// The minimum slot that the request can be evaluated at.
// 	// This parameter is optional.
// 	MinContextSlot *uint64
// }

// type Context struct {
// 	Slot uint64
// }

// // represents solana-go RPCContext
// type RPCContext struct {
// 	Context Context
// }

// type DataBytesOrJSON struct {
// 	RawDataEncoding EncodingType
// 	AsDecodedBinary []byte
// 	AsJSON          []byte
// }

// // represents solana-go Account
// type Account struct {
// 	// Number of lamports assigned to this account
// 	Lamports uint64

// 	// Pubkey of the program this account has been assigned to
// 	Owner PublicKey

// 	// Data associated with the account, either as encoded binary data or JSON format {<program>: <state>}, depending on encoding parameter
// 	Data *DataBytesOrJSON

// 	// Boolean indicating if the account contains a program (and is strictly read-only)
// 	Executable bool

// 	// The epoch at which this account will next owe rent
// 	RentEpoch *big.Int

// 	// The amount of storage space required to store the token account
// 	Space uint64
// }

// represents solana-go TransactionDetailsType
// type TransactionDetailsType string

// const (
// 	TransactionDetailsFull       TransactionDetailsType = "full"
// 	TransactionDetailsSignatures TransactionDetailsType = "signatures"
// 	TransactionDetailsNone       TransactionDetailsType = "none"
// 	TransactionDetailsAccounts   TransactionDetailsType = "accounts"
// )

// type TransactionVersion int

// const (
// 	LegacyTransactionVersion TransactionVersion = -1
// 	legacyVersion                               = `"legacy"`
// )

// type ConfirmationStatusType string

// const (
// 	ConfirmationStatusProcessed ConfirmationStatusType = "processed"
// 	ConfirmationStatusConfirmed ConfirmationStatusType = "confirmed"
// 	ConfirmationStatusFinalized ConfirmationStatusType = "finalized"
// )

// type TransactionWithMeta struct {
// 	// The slot this transaction was processed in.
// 	Slot uint64

// 	// Estimated production time, as Unix timestamp (seconds since the Unix epoch)
// 	// of when the transaction was processed.
// 	// Nil if not available.
// 	BlockTime *UnixTimeSeconds

// 	Transaction *DataBytesOrJSON
// 	// JSON encoded solana-go TransactionMeta
// 	MetaJSON []byte

// 	Version TransactionVersion
// }

// represents solana-go GetBlockOpts
// type GetBlockOpts struct {
// 	// Encoding for each returned Transaction, either "json", "jsonParsed", "base58" (slow), "base64".
// 	// If parameter not provided, the default encoding is "json".
// 	// - "jsonParsed" encoding attempts to use program-specific instruction parsers to return
// 	//   more human-readable and explicit data in the transaction.message.instructions list.
// 	// - If "jsonParsed" is requested but a parser cannot be found, the instruction falls back
// 	//   to regular JSON encoding (accounts, data, and programIdIndex fields).
// 	//
// 	// This parameter is optional.
// 	Encoding EncodingType

// 	// Level of transaction detail to return.
// 	// If parameter not provided, the default detail level is "full".
// 	//
// 	// This parameter is optional.
// 	TransactionDetails TransactionDetailsType

// 	// Whether to populate the rewards array.
// 	// If parameter not provided, the default includes rewards.
// 	//
// 	// This parameter is optional.
// 	Rewards *bool

// 	// "processed" is not supported.
// 	// If parameter not provided, the default is "finalized".
// 	//
// 	// This parameter is optional.
// 	Commitment CommitmentType

// 	// Max transaction version to return in responses.
// 	// If the requested block contains a transaction with a higher version, an error will be returned.
// 	MaxSupportedTransactionVersion *uint64
// }

// var (
// 	MaxSupportedTransactionVersion0 uint64 = 0
// 	MaxSupportedTransactionVersion1 uint64 = 1
// )

// // UnixTimeSeconds represents a UNIX second-resolution timestamp.
// type UnixTimeSeconds int64

// type GetMultipleAccountsOpts GetAccountInfoOpts
