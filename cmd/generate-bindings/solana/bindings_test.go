package solana_test

import (
	"context"
	"testing"

	"github.com/gagliardetto/solana-go"
	ocr3types "github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/types"
	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	realSolana "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
	realSolanaMock "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana/mock"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	consensusmock "github.com/smartcontractkit/cre-sdk-go/internal_testing/capabilities/consensus/mock"
	"github.com/test-go/testify/require"
	"google.golang.org/protobuf/proto"

	datastorage "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana/testdata/data_storage"
)

const anyChainSelector = uint64(1337)

func TestGeneratedBindingsCodec(t *testing.T) {
	codec := datastorage.Codec{}

	t.Run("encode functions", func(t *testing.T) {
		// structs
		userData := datastorage.UserData{
			Key:   "testKey",
			Value: "testValue",
		}
		_, err := codec.EncodeUserDataStruct(userData)
		require.NoError(t, err)

		testPrivKey, err := solana.NewRandomPrivateKey()
		require.NoError(t, err)
		testPubKey := testPrivKey.PublicKey()

		logAccess := datastorage.AccessLogged{
			Caller:  testPubKey,
			Message: "testMessage",
		}
		_, err = codec.EncodeAccessLoggedStruct(logAccess)
		require.NoError(t, err)

		readData := datastorage.DataAccount{
			Sender: testPubKey.String(),
			Key:    "testKey",
			Value:  "testValue",
		}
		_, err = codec.EncodeDataAccountStruct(readData)
		require.NoError(t, err)

		storeData := datastorage.DynamicEvent{
			Key:           "testKey",
			UserData:      userData,
			Sender:        testPubKey.String(),
			Metadata:      []byte("testMetadata"),
			MetadataArray: [][]byte{},
		}
		_, err = codec.EncodeDynamicEventStruct(storeData)
		require.NoError(t, err)

		storeUserData := datastorage.UpdateReserves{
			TotalMinted:  100,
			TotalReserve: uint64(200),
		}
		_, err = codec.EncodeUpdateReservesStruct(storeUserData)
		require.NoError(t, err)
	})
}

func TestWriteReportMethods(t *testing.T) {
	client := &realSolana.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client)
	require.NoError(t, err, "Failed to create DataStorage instance")

	report := ocr3types.Metadata{
		Version:          1,
		ExecutionID:      "1234567890123456789012345678901234567890123456789012345678901234",
		Timestamp:        1620000000,
		DONID:            1,
		DONConfigVersion: 1,
		WorkflowID:       "1234567890123456789012345678901234567890123456789012345678901234",
		WorkflowName:     "12",
		WorkflowOwner:    "1234567890123456789012345678901234567890",
		ReportID:         "1234",
	}

	rawReport, err := report.Encode()
	require.NoError(t, err)

	consensusCap, err := consensusmock.NewConsensusCapability(t)
	require.NoError(t, err, "Failed to create Consensus capability")
	consensusCap.Report = func(_ context.Context, input *sdk.ReportRequest) (*sdk.ReportResponse, error) {
		return &sdk.ReportResponse{
			RawReport: rawReport,
		}, nil
	}

	solanaCap, err := realSolanaMock.NewClientCapability(anyChainSelector, t)
	require.NoError(t, err, "Failed to create Solana client capability")
	solanaCap.WriteReport = func(_ context.Context, req *realSolana.WriteReportRequest) (*realSolana.WriteReportReply, error) {
		return &realSolana.WriteReportReply{
			TxStatus:    realSolana.TxStatus_TX_STATUS_SUCCESS,
			TxSignature: []byte{0x01, 0x02, 0x03, 0x04},
		}, nil
	}

	runtime := testutils.NewRuntime(t, testutils.Secrets{})

	reply := ds.WriteReportFromUserData(runtime, datastorage.UserData{
		Key:   "testKey",
		Value: "testValue",
	}, nil)
	require.NoError(t, err, "WriteReportDataStorageUserData should not return an error")
	response, err := reply.Await()
	require.NoError(t, err, "Awaiting WriteReportDataStorageUserData reply should not return an error")
	require.NotNil(t, response, "Response from WriteReportDataStorageUserData should not be nil")
	require.True(t, proto.Equal(&realSolana.WriteReportReply{
		TxStatus:    realSolana.TxStatus_TX_STATUS_SUCCESS,
		TxSignature: []byte{0x01, 0x02, 0x03, 0x04},
	}, response), "Response should match expected WriteReportReply")
}

func TestEncodeStruct(t *testing.T) {
	client := &realSolana.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client)
	require.NoError(t, err, "Failed to create DataStorage instance")

	str := datastorage.DataAccount{
		Key:    "testKey",
		Value:  "testValue",
		Sender: "testSender",
	}

	encoded, err := ds.Codec.EncodeDataAccountStruct(str)
	require.NoError(t, err, "Encoding DataStorageDataAccount should not return an error")
	require.NotNil(t, encoded, "Encoded data should not be nil")
}
