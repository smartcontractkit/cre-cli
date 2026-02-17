package solana_test

import (
	"context"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/require"
	"google.golang.org/protobuf/proto"

	ocr3types "github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/types"
	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	realSolana "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
	realSolanaMock "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana/mock"

	datastorage "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana/testdata/data_storage"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	consensusmock "github.com/smartcontractkit/cre-sdk-go/internal_testing/capabilities/consensus/mock"
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

		// onReport := datastorage.OnReportInput{
		// 	Metadata: []byte("testMetadata"),
		// 	Payload:  []byte("testPayload"),
		// }
		// _, err = codec.EncodeOnReportMethodCall(onReport)
		// require.NoError(t, err)
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

// func TestReadMethods(t *testing.T) {
// 	t.Run("single", func(t *testing.T) {
// 		client := &realSolana.Client{ChainSelector: anyChainSelector}
// 		ds, err := datastorage.NewDataStorage(client)
// 		require.NoError(t, err, "Failed to create DataStorage instance")

// 		// Encode the expected string response
// 		dataAccount := datastorage.DataAccount{
// 			Sender: "testSender",
// 			Key:    "testKey",
// 			Value:  "testValue",
// 		}
// 		encodedData, err := dataAccount.Marshal()
// 		require.NoError(t, err)

// 		dataAccountDiscriminator := datastorage.Account_DataAccount
// 		dataAccountDiscriminatorBytes := dataAccountDiscriminator[:]
// 		encodedData = append(dataAccountDiscriminatorBytes, encodedData...)

// 		solanaCap, err := realSolanaMock.NewClientCapability(anyChainSelector, t)
// 		require.NoError(t, err, "Failed to create EVM client capability")

// 		solanaCap.GetAccountInfoWithOpts = func(_ context.Context, input *realSolana.GetAccountInfoWithOptsRequest) (*realSolana.GetAccountInfoWithOptsReply, error) {
// 			reply := &realSolana.GetAccountInfoWithOptsReply{
// 				Value: &realSolana.Account{
// 					Data: &realSolana.DataBytesOrJSON{
// 						// AsDecodedBinary: encodedData,
// 						Body: &realSolana.DataBytesOrJSON_Raw{
// 							Raw: encodedData,
// 						},
// 					},
// 				},
// 			}
// 			return reply, nil
// 		}
// 		runtime := testutils.NewRuntime(t, testutils.Secrets{})
// 		randomAddress, err := solana.NewRandomPrivateKey()
// 		require.NoError(t, err)
// 		testBigInt := big.NewInt(123456)
// 		reply := ds.ReadAccount_DataAccount(runtime, randomAddress.PublicKey(), testBigInt)
// 		require.NotNil(t, reply, "ReadData should return a non-nil promise")

// 		resp, err := reply.Await()
// 		require.NoError(t, err, "Awaiting ReadData reply should not return an error")
// 		require.Equal(t, dataAccount.Value, resp.Value, "Decoded value should match expected string")
// 	})
// }

// func TestDecodeEvents(t *testing.T) {
// 	t.Run("Success", func(t *testing.T) {
// 		client := &realSolana.Client{ChainSelector: anyChainSelector}
// 		ds, err := datastorage.NewDataStorage(client)
// 		require.NoError(t, err, "Failed to create DataStorage instance")

// 		testPrivKey, err := solana.NewRandomPrivateKey()
// 		require.NoError(t, err)
// 		testPubKey := testPrivKey.PublicKey()
// 		testLog := datastorage.AccessLogged{
// 			Caller:  testPubKey,
// 			Message: "testMessage",
// 		}

// 		data, err := ds.Codec.EncodeAccessLoggedStruct(testLog)
// 		require.NoError(t, err)
// 		discriminator := datastorage.Event_AccessLogged

// 		log := &solanasdk.Log{
// 			Data: append(discriminator[:], data...),
// 		}

// 		out, err := ds.Codec.DecodeAccessLogged(log)
// 		require.NoError(t, err)
// 		require.Equal(t, testPubKey, out.Caller)
// 		require.Equal(t, "testMessage", out.Message)

// 		testLog2 := datastorage.DynamicEvent{
// 			Key: "testKey",
// 			UserData: datastorage.UserData{
// 				Key:   "testKey",
// 				Value: "testValue",
// 			},
// 			Sender:        testPubKey.String(),
// 			Metadata:      []byte("testMetadata"),
// 			MetadataArray: [][]byte{},
// 		}
// 		data2, err := ds.Codec.EncodeDynamicEventStruct(testLog2)
// 		require.NoError(t, err)
// 		discriminator2 := datastorage.Event_DynamicEvent
// 		log2 := &solanasdk.Log{
// 			Data: append(discriminator2[:], data2...),
// 		}
// 		out2, err := ds.Codec.DecodeDynamicEvent(log2)
// 		require.NoError(t, err)
// 		require.Equal(t, testPubKey.String(), out2.Sender)
// 		require.Equal(t, "testMetadata", string(out2.Metadata))
// 		require.Equal(t, "testKey", out2.Key)
// 		require.Equal(t, "testValue", out2.UserData.Value)
// 	})
// }

// func TestLogTrigger(t *testing.T) {
// 	client := &realSolana.Client{ChainSelector: anyChainSelector}
// 	ds, err := datastorage.NewDataStorage(client)
// 	require.NoError(t, err, "Failed to create DataStorage instance")
// 	t.Run("simple event", func(t *testing.T) {
// 		testPrivKey, err := solana.NewRandomPrivateKey()
// 		require.NoError(t, err)
// 		testPubKey := testPrivKey.PublicKey()
// 		events := []datastorage.AccessLogged{
// 			{
// 				Caller:  testPubKey,
// 				Message: "testMessage",
// 			},
// 		}

// 		encoded, err := ds.Codec.EncodeAccessLoggedStruct(events[0])
// 		require.NoError(t, err, "Encoding AccessLogged should not return an error")
// 		discriminator := datastorage.Event_AccessLogged
// 		encoded = append(discriminator[:], encoded...)

// 		trigger, err := ds.LogTrigger_AccessLogged(anyChainSelector, []solanasdk.SubKeyPathAndFilter{
// 			{
// 				SubkeyPath: "Caller",
// 				Value:      testPubKey,
// 			},
// 		})
// 		require.NotNil(t, trigger)
// 		require.NoError(t, err)

// 		// Create a mock log that simulates what would be returned by the blockchain
// 		mockLog := &solanasdk.Log{
// 			Address: solanatypes.PublicKey(datastorage.ProgramID),
// 			Data:    encoded,
// 		}

// 		// Call Adapt to decode the log
// 		decodedLog, err := trigger.Adapt(mockLog)
// 		require.NoError(t, err, "Adapt should not return an error")
// 		require.NotNil(t, decodedLog, "Decoded log should not be nil")
// 		require.Equal(t, events[0].Caller, decodedLog.Data.Caller, "Decoded caller should match")
// 		require.Equal(t, events[0].Message, decodedLog.Data.Message, "Decoded message should match")
// 	})

// 	t.Run("dynamic event", func(t *testing.T) {
// 		testPrivKey, err := solana.NewRandomPrivateKey()
// 		require.NoError(t, err)
// 		testPubKey := testPrivKey.PublicKey()
// 		events := []datastorage.DynamicEvent{
// 			{
// 				Key: "testKey",
// 				UserData: datastorage.UserData{
// 					Key:   "testKey",
// 					Value: "testValue",
// 				},
// 				Sender:        testPubKey.String(),
// 				Metadata:      []byte("testMetadata"),
// 				MetadataArray: [][]byte{},
// 			},
// 		}

// 		encoded, err := ds.Codec.EncodeDynamicEventStruct(events[0])
// 		require.NoError(t, err, "Encoding DynamicEvent should not return an error")
// 		discriminator := datastorage.Event_DynamicEvent
// 		encoded = append(discriminator[:], encoded...)

// 		trigger, err := ds.LogTrigger_DynamicEvent(anyChainSelector, []solanasdk.SubKeyPathAndFilter{
// 			{
// 				SubkeyPath: "UserData.Key",
// 				Value:      "testKey",
// 			},
// 			{
// 				SubkeyPath: "Key",
// 				Value:      "testKey",
// 			},
// 		})
// 		require.NotNil(t, trigger)
// 		require.NoError(t, err)

// 		// Create a mock log that simulates what would be returned by the blockchain
// 		mockLog := &solanasdk.Log{
// 			Address: solanatypes.PublicKey(datastorage.ProgramID),
// 			Data:    encoded,
// 		}

// 		// Call Adapt to decode the log
// 		decodedLog, err := trigger.Adapt(mockLog)
// 		require.NoError(t, err, "Adapt should not return an error")
// 		require.NotNil(t, decodedLog, "Decoded log should not be nil")
// 		require.Equal(t, events[0].Key, decodedLog.Data.Key, "Decoded key should match")
// 		require.Equal(t, events[0].UserData.Key, decodedLog.Data.UserData.Key, "Decoded user data key should match")
// 		require.Equal(t, events[0].UserData.Value, decodedLog.Data.UserData.Value, "Decoded user data value should match")
// 		require.Equal(t, events[0].Sender, decodedLog.Data.Sender, "Decoded sender should match")
// 		require.Equal(t, events[0].Metadata, decodedLog.Data.Metadata, "Decoded metadata should match")
// 	})
// }
