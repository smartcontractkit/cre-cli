package solana_bindings_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/test-go/testify/require"
	"google.golang.org/protobuf/proto"

	ocr3types "github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/types"
	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	solanasdk "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/capabilities/blockchain/solana"
	solanamock "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/capabilities/blockchain/solana/mock"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/common"
	solanatypes "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/types"
	datastorage "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/data_storage"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	consensusmock "github.com/smartcontractkit/cre-sdk-go/internal_testing/capabilities/consensus/mock"
)

const anyChainSelector = uint64(1337)

func TestSolanaBasic(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)
	version, err := solanaClient.GetVersion(context.Background())
	require.NoError(t, err)
	fmt.Println("version", version)
	health, err := solanaClient.GetHealth(context.Background())
	require.NoError(t, err)
	fmt.Println("health", health)
	// fmt.Println(my_anchor_project.ProgramID.String())
}

func TestSolanaInit(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// dataAccountAccount, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	my_anchor_project.ProgramID,
	// )
	// ix, err := my_anchor_project.NewInitializeInstruction(
	// 	"test-data",
	// 	dataAccountAccount,
	// 	pk.PublicKey(),
	// 	solana.SystemProgramID,
	// )
	require.NoError(t, err)

	res, err := common.SendAndConfirm(
		context.Background(),
		solanaClient,
		[]solana.Instruction{},
		pk,
		rpc.CommitmentConfirmed,
		common.AddSigners(pk),
	)
	require.NoError(t, err)
	fmt.Println("res", res.Meta.LogMessages)

}

func TestSolanaGetData(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// dataAccountAccount, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	my_anchor_project.ProgramID,
	// )

	// ix3, err := my_anchor_project.NewGetInputDataInstruction("test-data")
	require.NoError(t, err)
	// ix4, err := my_anchor_project.NewGetInputDataFromAccountInstruction("test-data", dataAccountAccount)
	// require.NoError(t, err)
	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3, ix4}, pk, rpc.CommitmentConfirmed)
	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{}, pk, rpc.CommitmentConfirmed)

	require.NoError(t, err)
	for _, log := range res.Meta.LogMessages {
		if strings.Contains(log, "Program log:") {
			fmt.Println("log", log)
		}
	}
}

func TestSolanaReadAccount(t *testing.T) {
	// create client
	// solanaClient := rpc.New("http://localhost:8899")
	// // find pda
	// dataAccountAddress, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	// my_anchor_project.ProgramID,
	// )
	// // call rpc
	// resp, err := solanaClient.GetAccountInfoWithOpts(
	// 	context.Background(),
	// 	dataAccountAddress,
	// 	&rpc.GetAccountInfoOpts{
	// 		Commitment: rpc.CommitmentConfirmed,
	// 	},
	// )
	// require.NoError(t, err, "failed to get account info")
	// // parse account info
	// // data, err := my_anchor_project.ParseAccount_DataAccount(resp.Value.Data.GetBinary())
	// require.NoError(t, err, "failed to parse account info")
	// fmt.Println("data", data)

	// data2, err := my_anchor_project.ReadAccount_DataAccount([][]byte{[]byte("test")}, solanaClient)
	// require.NoError(t, err, "failed to read account info")
	// fmt.Println("data2", data2)
}

func TestSolanaWriteAccount(t *testing.T) {
	// solanaClient := rpc.New("http://localhost:8899")
	// pk, err := solana.NewRandomPrivateKey()
	// require.NoError(t, err)
	// common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// // dataAccountAddress, _, err := solana.FindProgramAddress(
	// // 	[][]byte{[]byte("test")},
	// // 	// my_anchor_project.ProgramID,
	// // )
	// // ix, err := my_anchor_project.NewUpdateDataInstruction("test-data-new", dataAccountAddress)
	// require.NoError(t, err)

	// // ix2, err := my_anchor_project.NewUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress)
	// // require.NoError(t, err)

	// // res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix, ix2}, pk, rpc.CommitmentConfirmed)
	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix}, pk, rpc.CommitmentConfirmed)

	// require.NoError(t, err)
	// fmt.Println("res", res.Meta.LogMessages)

	// // output, err := common.ExtractTypedReturnValue(context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String(), func(b []byte) string {
	// // 	require.Len(t, b, int(binary.LittleEndian.Uint32(b[:4]))+4) // the first 4 bytes just encodes the length
	// // 	return string(b[4:])
	// // })
	// require.NoError(t, err)
	// fmt.Println("output", output)

	// output2, err := common.ExtractAnchorTypedReturnValue[my_anchor_project.UpdateResponse](context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String())
	// require.NoError(t, err)
	// fmt.Println("output2", output2)

	// output3, err := my_anchor_project.SendUpdateDataInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("output3", output3)

	// output4, err := my_anchor_project.SendUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("output4", output4.Data)
}

/*
anchor-go \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/my_anchor_project.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project \
  --program-id 2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN \
  --no-go-mod

./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/my_project.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project \
  --program-id 2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN \
  --no-go-mod

	 go build -ldflags "-w" -o anchor;./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/data_storage.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/data_storage \
  --program-id ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL \
  --no-go-mod

*/

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

func TestDecodeEvents(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		client := &solanasdk.Client{ChainSelector: anyChainSelector}
		ds, err := datastorage.NewDataStorage(client)
		require.NoError(t, err, "Failed to create DataStorage instance")

		testPrivKey, err := solana.NewRandomPrivateKey()
		require.NoError(t, err)
		testPubKey := testPrivKey.PublicKey()
		testLog := datastorage.AccessLogged{
			Caller:  testPubKey,
			Message: "testMessage",
		}

		data, err := ds.Codec.EncodeAccessLoggedStruct(testLog)
		require.NoError(t, err)
		discriminator := datastorage.Event_AccessLogged

		log := &solanasdk.Log{
			Data: append(discriminator[:], data...),
		}

		out, err := ds.Codec.DecodeAccessLogged(log)
		require.NoError(t, err)
		require.Equal(t, testPubKey, out.Caller)
		require.Equal(t, "testMessage", out.Message)

		testLog2 := datastorage.DynamicEvent{
			Key: "testKey",
			UserData: datastorage.UserData{
				Key:   "testKey",
				Value: "testValue",
			},
			Sender:        testPubKey.String(),
			Metadata:      []byte("testMetadata"),
			MetadataArray: [][]byte{},
		}
		data2, err := ds.Codec.EncodeDynamicEventStruct(testLog2)
		require.NoError(t, err)
		discriminator2 := datastorage.Event_DynamicEvent
		log2 := &solanasdk.Log{
			Data: append(discriminator2[:], data2...),
		}
		out2, err := ds.Codec.DecodeDynamicEvent(log2)
		require.NoError(t, err)
		require.Equal(t, testPubKey.String(), out2.Sender)
		require.Equal(t, "testMetadata", string(out2.Metadata))
		require.Equal(t, "testKey", out2.Key)
		require.Equal(t, "testValue", out2.UserData.Value)
	})
}

func TestReadMethods(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		client := &solanasdk.Client{ChainSelector: anyChainSelector}
		ds, err := datastorage.NewDataStorage(client)
		require.NoError(t, err, "Failed to create DataStorage instance")

		// Encode the expected string response
		dataAccount := datastorage.DataAccount{
			Sender: "testSender",
			Key:    "testKey",
			Value:  "testValue",
		}
		encodedData, err := dataAccount.Marshal()
		require.NoError(t, err)

		dataAccountDiscriminator := datastorage.Account_DataAccount
		dataAccountDiscriminatorBytes := dataAccountDiscriminator[:]
		encodedData = append(dataAccountDiscriminatorBytes, encodedData...)

		solanaCap, err := solanamock.NewClientCapability(anyChainSelector, t)
		require.NoError(t, err, "Failed to create EVM client capability")

		solanaCap.GetAccountInfoWithOpts = func(_ context.Context, input *solanasdk.GetAccountInfoRequest) (*solanasdk.GetAccountInfoReply, error) {
			reply := &solanasdk.GetAccountInfoReply{
				Value: &solanasdk.Account{
					Data: &solanasdk.DataBytesOrJSON{
						AsDecodedBinary: encodedData,
					},
				},
			}
			return reply, nil
		}
		runtime := testutils.NewRuntime(t, testutils.Secrets{})
		randomAddress, err := solana.NewRandomPrivateKey()
		require.NoError(t, err)
		testBigInt := big.NewInt(123456)
		reply := ds.ReadAccount_DataAccount(runtime, randomAddress.PublicKey(), testBigInt)
		require.NotNil(t, reply, "ReadData should return a non-nil promise")

		resp, err := reply.Await()
		require.NoError(t, err, "Awaiting ReadData reply should not return an error")
		require.Equal(t, dataAccount.Value, resp.Value, "Decoded value should match expected string")
	})
}

func TestWriteReportMethods(t *testing.T) {
	client := &solanasdk.Client{ChainSelector: anyChainSelector}
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

	solanaCap, err := solanamock.NewClientCapability(anyChainSelector, t)
	require.NoError(t, err, "Failed to create Solana client capability")
	solanaCap.WriteReport = func(_ context.Context, req *solanasdk.WriteCreReportRequest) (*solanasdk.WriteReportReply, error) {
		return &solanasdk.WriteReportReply{
			TxStatus: solanasdk.TxStatus_TX_STATUS_SUCCESS,
			TxHash:   []byte{0x01, 0x02, 0x03, 0x04},
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
	require.True(t, proto.Equal(&solanasdk.WriteReportReply{
		TxStatus: solanasdk.TxStatus_TX_STATUS_SUCCESS,
		TxHash:   []byte{0x01, 0x02, 0x03, 0x04},
	}, response), "Response should match expected WriteReportReply")
}

func TestEncodeStruct(t *testing.T) {
	client := &solanasdk.Client{ChainSelector: anyChainSelector}
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

func TestLogTrigger(t *testing.T) {
	client := &solanasdk.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client)
	require.NoError(t, err, "Failed to create DataStorage instance")
	t.Run("simple event", func(t *testing.T) {
		testPrivKey, err := solana.NewRandomPrivateKey()
		require.NoError(t, err)
		testPubKey := testPrivKey.PublicKey()
		events := []datastorage.AccessLogged{
			{
				Caller:  testPubKey,
				Message: "testMessage",
			},
		}

		encoded, err := ds.Codec.EncodeAccessLoggedStruct(events[0])
		require.NoError(t, err, "Encoding AccessLogged should not return an error")
		discriminator := datastorage.Event_AccessLogged
		encoded = append(discriminator[:], encoded...)

		trigger, err := ds.LogTrigger_AccessLogged(anyChainSelector, []solanasdk.SubKeyPathAndFilter{
			{
				SubkeyPath: "Caller",
				Value:      testPubKey,
			},
		})
		require.NotNil(t, trigger)
		require.NoError(t, err)

		// Create a mock log that simulates what would be returned by the blockchain
		mockLog := &solanasdk.Log{
			Address: solanatypes.PublicKey(datastorage.ProgramID),
			Data:    encoded,
		}

		// Call Adapt to decode the log
		decodedLog, err := trigger.Adapt(mockLog)
		require.NoError(t, err, "Adapt should not return an error")
		require.NotNil(t, decodedLog, "Decoded log should not be nil")
		require.Equal(t, events[0].Caller, decodedLog.Data.Caller, "Decoded caller should match")
		require.Equal(t, events[0].Message, decodedLog.Data.Message, "Decoded message should match")
	})

	t.Run("dynamic event", func(t *testing.T) {
		testPrivKey, err := solana.NewRandomPrivateKey()
		require.NoError(t, err)
		testPubKey := testPrivKey.PublicKey()
		events := []datastorage.DynamicEvent{
			{
				Key: "testKey",
				UserData: datastorage.UserData{
					Key:   "testKey",
					Value: "testValue",
				},
				Sender:        testPubKey.String(),
				Metadata:      []byte("testMetadata"),
				MetadataArray: [][]byte{},
			},
		}

		encoded, err := ds.Codec.EncodeDynamicEventStruct(events[0])
		require.NoError(t, err, "Encoding DynamicEvent should not return an error")
		discriminator := datastorage.Event_DynamicEvent
		encoded = append(discriminator[:], encoded...)

		trigger, err := ds.LogTrigger_DynamicEvent(anyChainSelector, []solanasdk.SubKeyPathAndFilter{
			{
				SubkeyPath: "UserData.Key",
				Value:      "testKey",
			},
			{
				SubkeyPath: "Key",
				Value:      "testKey",
			},
		})
		require.NotNil(t, trigger)
		require.NoError(t, err)

		// Create a mock log that simulates what would be returned by the blockchain
		mockLog := &solanasdk.Log{
			Address: solanatypes.PublicKey(datastorage.ProgramID),
			Data:    encoded,
		}

		// Call Adapt to decode the log
		decodedLog, err := trigger.Adapt(mockLog)
		require.NoError(t, err, "Adapt should not return an error")
		require.NotNil(t, decodedLog, "Decoded log should not be nil")
		require.Equal(t, events[0].Key, decodedLog.Data.Key, "Decoded key should match")
		require.Equal(t, events[0].UserData.Key, decodedLog.Data.UserData.Key, "Decoded user data key should match")
		require.Equal(t, events[0].UserData.Value, decodedLog.Data.UserData.Value, "Decoded user data value should match")
		require.Equal(t, events[0].Sender, decodedLog.Data.Sender, "Decoded sender should match")
		require.Equal(t, events[0].Metadata, decodedLog.Data.Metadata, "Decoded metadata should match")
	})
}
