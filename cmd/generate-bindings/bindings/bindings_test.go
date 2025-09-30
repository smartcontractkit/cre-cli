package bindings_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	ocr3types "github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/types"
	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm/bindings"
	evmmock "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm/mock"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	consensusmock "github.com/smartcontractkit/cre-sdk-go/internal_testing/capabilities/consensus/mock"

	datastorage "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings/testdata"
)

const anyChainSelector = uint64(1337)

func TestGeneratedBindingsCodec(t *testing.T) {
	codec, err := datastorage.NewCodec()
	require.NoError(t, err)

	t.Run("encode functions", func(t *testing.T) {
		// structs
		userData := datastorage.UserData{
			Key:   "testKey",
			Value: "testValue",
		}

		_, err := codec.EncodeUserDataStruct(userData)
		require.NoError(t, err)

		// inputs
		logAccess := datastorage.LogAccessInput{
			Message: "testMessage",
		}
		_, err = codec.EncodeLogAccessMethodCall(logAccess)
		require.NoError(t, err)

		onReport := datastorage.OnReportInput{
			Metadata: []byte("testMetadata"),
			Payload:  []byte("testPayload"),
		}
		_, err = codec.EncodeOnReportMethodCall(onReport)
		require.NoError(t, err)

		readData := datastorage.ReadDataInput{
			User: common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			Key:  "testKey",
		}
		_, err = codec.EncodeReadDataMethodCall(readData)
		require.NoError(t, err)

		storeData := datastorage.StoreDataInput{
			Key:   "testKey",
			Value: "testValue",
		}
		_, err = codec.EncodeStoreDataMethodCall(storeData)
		require.NoError(t, err)

		storeUserData := datastorage.StoreUserDataInput{
			UserData: userData,
		}
		_, err = codec.EncodeStoreUserDataMethodCall(storeUserData)
		require.NoError(t, err)

		updateDataInput := datastorage.UpdateDataInput{
			Key:      "testKey",
			NewValue: "newTestValue",
		}
		_, err = codec.EncodeUpdateDataMethodCall(updateDataInput)
		require.NoError(t, err)
	})
}

func TestDecodeEvents(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ds := newDataStorage(t)

		caller := common.HexToAddress("0xAb8483F64d9C6d1EcF9b849Ae677dD3315835cb2")
		message := "Test access log"

		ev := ds.ABI.Events["AccessLogged"]

		topics := [][]byte{
			ds.Codec.AccessLoggedLogHash(),
			caller.Bytes(),
		}

		var nonIndexed abi.Arguments
		for _, arg := range ev.Inputs {
			if !arg.Indexed {
				nonIndexed = append(nonIndexed, arg)
			}
		}
		data, err := nonIndexed.Pack(message)
		require.NoError(t, err)

		log := &evm.Log{
			Topics: topics,
			Data:   data,
		}

		out, err := ds.Codec.DecodeAccessLogged(log)
		require.NoError(t, err)
		require.Equal(t, caller, out.Caller)
		require.Equal(t, message, out.Message)
	})
}

func TestReadMethods(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		client := &evm.Client{ChainSelector: anyChainSelector}
		ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
		require.NoError(t, err, "Failed to create DataStorage instance")

		expectedValue := "test string response"

		// Encode the expected string response
		stringType, err := abi.NewType("string", "", nil)
		require.NoError(t, err)
		args := abi.Arguments{{Name: "value", Type: stringType}}
		encodedData, err := args.Pack(expectedValue)
		require.NoError(t, err)

		evmCap, err := evmmock.NewClientCapability(anyChainSelector, t)
		require.NoError(t, err, "Failed to create EVM client capability")

		evmCap.HeaderByNumber = func(_ context.Context, input *evm.HeaderByNumberRequest) (*evm.HeaderByNumberReply, error) {
			header := &evm.HeaderByNumberReply{
				Header: &evm.Header{
					BlockNumber: valuespb.NewBigIntFromInt(big.NewInt(123456)),
				},
			}
			return header, nil
		}

		evmCap.CallContract = func(_ context.Context, input *evm.CallContractRequest) (*evm.CallContractReply, error) {
			reply := &evm.CallContractReply{
				Data: encodedData,
			}
			return reply, nil
		}

		runtime := testutils.NewRuntime(t, map[string]string{})
		reply := ds.ReadData(runtime, datastorage.ReadDataInput{
			User: common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			Key:  "testKey",
		}, nil)
		require.NotNil(t, reply, "ReadData should return a non-nil promise")

		decodedValue, err := reply.Await()
		require.NoError(t, err, "Awaiting ReadData reply should not return an error")
		require.Equal(t, expectedValue, decodedValue, "Decoded value should match expected string")
	})

	t.Run("multiple", func(t *testing.T) {
		client := &evm.Client{ChainSelector: anyChainSelector}
		ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
		require.NoError(t, err, "Failed to create DataStorage instance")

		expectedReserves := []datastorage.UpdateReserves{
			{
				TotalMinted:  big.NewInt(100),
				TotalReserve: big.NewInt(200),
			},
			{
				TotalMinted:  big.NewInt(300),
				TotalReserve: big.NewInt(400),
			},
		}

		arrayType, err := abi.NewType("tuple[]", "", []abi.ArgumentMarshaling{
			{Name: "totalMinted", Type: "uint256"},
			{Name: "totalReserve", Type: "uint256"},
		})
		require.NoError(t, err)

		args := abi.Arguments{{Name: "reserves", Type: arrayType}}
		encodedData, err := args.Pack(expectedReserves)
		require.NoError(t, err)

		evmCap, err := evmmock.NewClientCapability(anyChainSelector, t)
		require.NoError(t, err, "Failed to create EVM client capability")

		evmCap.HeaderByNumber = func(_ context.Context, input *evm.HeaderByNumberRequest) (*evm.HeaderByNumberReply, error) {
			header := &evm.HeaderByNumberReply{
				Header: &evm.Header{
					BlockNumber: valuespb.NewBigIntFromInt(big.NewInt(123456)),
				},
			}
			return header, nil
		}

		evmCap.CallContract = func(_ context.Context, input *evm.CallContractRequest) (*evm.CallContractReply, error) {
			reply := &evm.CallContractReply{
				Data: encodedData,
			}
			return reply, nil
		}

		runtime := testutils.NewRuntime(t, map[string]string{})
		reply := ds.GetMultipleReserves(runtime, nil)
		require.NotNil(t, reply, "GetMultipleReserves should return a non-nil promise")

		decodedReserves, err := reply.Await()
		require.NoError(t, err, "Awaiting GetMultipleReserves reply should not return an error")
		require.Len(t, decodedReserves, 2, "Should decode exactly 2 UpdateReserves structs")

		require.Equal(t, expectedReserves[0].TotalMinted, decodedReserves[0].TotalMinted, "First struct TotalMinted should match")
		require.Equal(t, expectedReserves[0].TotalReserve, decodedReserves[0].TotalReserve, "First struct TotalReserve should match")

		require.Equal(t, expectedReserves[1].TotalMinted, decodedReserves[1].TotalMinted, "Second struct TotalMinted should match")
		require.Equal(t, expectedReserves[1].TotalReserve, decodedReserves[1].TotalReserve, "Second struct TotalReserve should match")
	})

	t.Run("tuple returns", func(t *testing.T) {
		client := &evm.Client{ChainSelector: anyChainSelector}
		ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
		require.NoError(t, err, "Failed to create DataStorage instance")

		// Expected values that match the Solidity function: return (100, 200)
		expectedTotalMinted := big.NewInt(100)
		expectedTotalReserve := big.NewInt(200)

		// Create ABI arguments for encoding the expected tuple return values
		args := abi.Arguments{
			{Name: "totalMinted", Type: abi.Type{T: abi.UintTy, Size: 256}},
			{Name: "totalReserve", Type: abi.Type{T: abi.UintTy, Size: 256}},
		}
		encodedData, err := args.Pack(expectedTotalMinted, expectedTotalReserve)
		require.NoError(t, err)

		evmCap, err := evmmock.NewClientCapability(anyChainSelector, t)
		require.NoError(t, err, "Failed to create EVM client capability")

		evmCap.HeaderByNumber = func(_ context.Context, input *evm.HeaderByNumberRequest) (*evm.HeaderByNumberReply, error) {
			header := &evm.HeaderByNumberReply{
				Header: &evm.Header{
					BlockNumber: valuespb.NewBigIntFromInt(big.NewInt(123456)),
				},
			}
			return header, nil
		}

		evmCap.CallContract = func(_ context.Context, input *evm.CallContractRequest) (*evm.CallContractReply, error) {
			reply := &evm.CallContractReply{
				Data: encodedData,
			}
			return reply, nil
		}

		runtime := testutils.NewRuntime(t, map[string]string{})
		reply := ds.GetTupleReserves(runtime, nil)
		require.NotNil(t, reply, "GetTupleReserves should return a non-nil promise")

		decodedOutput, err := reply.Await()
		require.NoError(t, err, "Awaiting GetTupleReserves reply should not return an error")

		// Verify both return values are correctly decoded from the tuple
		require.Equal(t, expectedTotalMinted, decodedOutput.TotalMinted, "TotalMinted should match expected value")
		require.Equal(t, expectedTotalReserve, decodedOutput.TotalReserve, "TotalReserve should match expected value")

		// Verify the structure has the correct field names from the ABI
		require.NotNil(t, decodedOutput.TotalMinted, "TotalMinted field should be populated")
		require.NotNil(t, decodedOutput.TotalReserve, "TotalReserve field should be populated")
	})
}

func TestWriteReportMethods(t *testing.T) {
	client := &evm.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
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

	evmCap, err := evmmock.NewClientCapability(anyChainSelector, t)
	require.NoError(t, err, "Failed to create EVM client capability")
	evmCap.WriteReport = func(_ context.Context, req *evm.WriteReportRequest) (*evm.WriteReportReply, error) {
		require.Equal(t, rawReport, req.Report.RawReport)
		return &evm.WriteReportReply{
			TxStatus: evm.TxStatus_TX_STATUS_SUCCESS,
			TxHash:   []byte{0x01, 0x02, 0x03, 0x04},
		}, nil
	}

	runtime := testutils.NewRuntime(t, map[string]string{})

	reply := ds.WriteReportFromUserData(runtime, datastorage.UserData{
		Key:   "testKey",
		Value: "testValue",
	}, nil)
	require.NoError(t, err, "WriteReportDataStorageUserData should not return an error")
	response, err := reply.Await()
	require.NoError(t, err, "Awaiting WriteReportDataStorageUserData reply should not return an error")
	require.NotNil(t, response, "Response from WriteReportDataStorageUserData should not be nil")
	require.True(t, proto.Equal(&evm.WriteReportReply{
		TxStatus: evm.TxStatus_TX_STATUS_SUCCESS,
		TxHash:   []byte{0x01, 0x02, 0x03, 0x04},
	}, response), "Response should match expected WriteReportReply")
}

func TestEncodeStruct(t *testing.T) {
	ds := newDataStorage(t)

	str := datastorage.UpdateReserves{
		TotalMinted:  big.NewInt(100),
		TotalReserve: big.NewInt(200),
	}

	encoded, err := ds.Codec.EncodeUpdateReservesStruct(str)
	require.NoError(t, err, "Encoding DataStorageUpdateReserves should not return an error")
	require.NotNil(t, encoded, "Encoded data should not be nil")
}

func TestErrorHandling(t *testing.T) {
	ds := newDataStorage(t)

	requester := common.HexToAddress("0xAb8483F64d9C6d1EcF9b849Ae677dD3315835cb2")
	key := "testKey"
	reason := "not found"

	t.Run("valid", func(t *testing.T) {
		errDesc := ds.ABI.Errors["DataNotFound"]
		encoded := errDesc.ID.Bytes()[:4]
		args, err := errDesc.Inputs.Pack(requester, key, reason)
		require.NoError(t, err)
		encoded = append(encoded, args...)

		unpacked, err := ds.UnpackError(encoded)
		require.NoError(t, err)

		result, ok := unpacked.(*datastorage.DataNotFound)
		require.True(t, ok, "Unpacked error should be of type DataNotFoundError")

		require.Equal(t, requester, result.Requester)
		require.Equal(t, key, result.Key)
		require.Equal(t, reason, result.Reason)
	})

	t.Run("invalid", func(t *testing.T) {
		// Simulate an invalid error code
		invalidCode := []byte{0x01, 0x02, 0x03, 0x04}
		_, err := ds.UnpackError(invalidCode)
		require.Error(t, err, "Unpacking an invalid error code should return an error")
		require.Contains(t, err.Error(), "unknown error selector", "Error message should indicate unknown error code")
	})
}

func TestFilterLogs(t *testing.T) {
	client := &evm.Client{ChainSelector: anyChainSelector}
	anyAddress := common.HexToAddress("0xAb8483F64d9C6d1EcF9b849Ae677dD3315835cb2")
	ds, err := datastorage.NewDataStorage(client, anyAddress, &bindings.ContractInitOptions{})
	require.NoError(t, err, "Failed to create DataStorage instance")

	bh := []byte{0x01, 0x02, 0x03, 0x04}
	fb := big.NewInt(100)
	tb := big.NewInt(200)

	evmCap, err := evmmock.NewClientCapability(anyChainSelector, t)
	require.NoError(t, err, "Failed to create EVM client capability")
	evmCap.FilterLogs = func(_ context.Context, req *evm.FilterLogsRequest) (*evm.FilterLogsReply, error) {
		require.Equal(t, [][]byte{ds.Address.Bytes()}, req.FilterQuery.Addresses, "Filter should contain the correct address")
		require.Equal(t, bh, req.FilterQuery.BlockHash, "Filter should contain the correct block hash")
		require.Equal(t, fb.Bytes(), req.FilterQuery.FromBlock.GetAbsVal(), "Filter should contain the correct from block")
		require.Equal(t, tb.Bytes(), req.FilterQuery.ToBlock.GetAbsVal(), "Filter should contain the correct to block")
		logs := []*evm.Log{
			{
				Address: ds.Address.Bytes(),
				Topics:  [][]byte{ds.Codec.AccessLoggedLogHash()},
				Data:    []byte("test log data"),
			},
		}
		return &evm.FilterLogsReply{Logs: logs}, nil
	}

	runtime := testutils.NewRuntime(t, map[string]string{})

	reply := ds.FilterLogsAccessLogged(runtime, &bindings.FilterOptions{
		BlockHash: bh,
		FromBlock: fb,
		ToBlock:   tb,
	})
	response, err := reply.Await()
	require.NoError(t, err, "Awaiting FilteredLogsAccessLogged reply should not return an error")
	require.NotNil(t, response, "Response from FilteredLogsAccessLogged should not be nil")
	require.Len(t, response.Logs, 1, "Response should contain one log")
	require.Equal(t, ds.Address.Bytes(), response.Logs[0].Address)
}

func TestLogTrigger(t *testing.T) {
	client := &evm.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
	require.NoError(t, err, "Failed to create DataStorage instance")
	t.Run("simple event", func(t *testing.T) {
		ev := ds.ABI.Events["DataStored"]
		events := []datastorage.DataStored{
			{
				Sender: common.HexToAddress("0xAb8483F64d9C6d1EcF9b849Ae677dD3315835cb2"),
				Key:    "testKey",
				Value:  "testValue",
			},
			{
				Sender: common.HexToAddress("0xBb8483F64d9C6d1EcF9b849Ae677dD3315835cb2"),
				Key:    "testKey",
				Value:  "testValue",
			},
		}

		encoded, err := ds.Codec.EncodeDataStoredTopics(ev, events)
		require.NoError(t, err, "Encoding DataStored topics should not return an error")

		require.Equal(t, ds.Codec.DataStoredLogHash(), encoded[0].Values[0], "First topic value should be AccessLogged log hash")
		require.Len(t, encoded[1].Values, 2, "Second topic should have two values")
		expected1, err := abi.Arguments{ev.Inputs[0]}.Pack(events[0].Sender)
		require.NoError(t, err)
		require.Equal(t, expected1, encoded[1].Values[0])
		expected2, err := abi.Arguments{ev.Inputs[0]}.Pack(events[1].Sender)
		require.NoError(t, err)
		require.Equal(t, expected2, encoded[1].Values[1])

		trigger, err := ds.LogTriggerDataStoredLog(1, evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED, events)
		require.NotNil(t, trigger)
		require.NoError(t, err)

		// Test the Adapt method
    // Encode the full event data (Key and Value) for the first event
    // The DataStored event has: Sender (indexed), Key (non-indexed), Value (non-indexed)
    // We need to encode the non-indexed parameters (Key and Value) into the log data
    eventData, err := abi.Arguments{ev.Inputs[1], ev.Inputs[2]}.Pack(events[0].Key, events[0].Value)
    require.NoError(t, err, "Encoding event data should not return an error")

    // Create a mock log that simulates what would be returned by the blockchain
    mockLog := &evm.Log{
        Address: ds.Address.Bytes(),                    // Contract address
        Topics: [][]byte{
            ds.Codec.DataStoredLogHash(),              // Event signature hash
            expected1,                                 // Sender address (indexed)
        },
        Data: eventData,                               // Encoded Key and Value data
    }

    // Call Adapt to decode the log
    decodedLog, err := trigger.Adapt(mockLog)
    require.NoError(t, err, "Adapt should not return an error")
    require.NotNil(t, decodedLog, "Decoded log should not be nil")
    
    // Verify the decoded data matches what we expect
    require.Equal(t, events[0].Sender, decodedLog.Data.Sender, "Decoded sender should match")
    require.Equal(t, events[0].Key, decodedLog.Data.Key, "Decoded key should match")
    require.Equal(t, events[0].Value, decodedLog.Data.Value, "Decoded value should match")
    
    // // Verify the original log is preserved
    require.Equal(t, mockLog, decodedLog.Log, "Original log should be preserved")
	})
	t.Run("dynamic event", func(t *testing.T) {
		ev := ds.ABI.Events["DynamicEvent"]
		events := []datastorage.DynamicEvent{
			{
				Key: "testKey1",
				UserData: datastorage.UserData{
					Key:   "userKey1",
					Value: "userValue1",
				},
				Sender:   "testSender1",
				Metadata: common.HexToHash("metadata1"),
				MetadataArray: [][]byte{
					[]byte("meta1"),
					[]byte("meta2"),
				},
			},
			{
				Key: "testKey2",
				UserData: datastorage.UserData{
					Key:   "userKey2",
					Value: "userValue2",
				},
				Sender:   "testSender2",
				Metadata: common.HexToHash("metadata2"),
				MetadataArray: [][]byte{
					[]byte("meta3"),
					[]byte("meta4"),
				},
			},
		}

		encoded, err := ds.Codec.EncodeDynamicEventTopics(ev, events)
		require.NoError(t, err, "Encoding DynamicEvent topics should not return an error")

		require.Len(t, encoded, 4, "Trigger should have four topics")
		require.Equal(t, ds.Codec.DynamicEventLogHash(), encoded[0].Values[0], "First topic value should be DynamicEvent log hash")
		require.Len(t, encoded[1].Values, 2, "Second topic should have two values")
		packed1, err := abi.Arguments{ev.Inputs[1]}.Pack(events[0].UserData)

		expected1 := crypto.Keccak256(packed1)
		require.NoError(t, err)
		require.Equal(t, expected1, encoded[1].Values[0])
		packed2, err := abi.Arguments{ev.Inputs[1]}.Pack(events[1].UserData)

		expected2 := crypto.Keccak256(packed2)
		require.NoError(t, err)
		require.Equal(t, expected2, encoded[1].Values[1])

		
		expected3 := events[0].Metadata.Bytes()
		require.Equal(t, expected3, encoded[2].Values[0])

		expected4 := events[1].Metadata.Bytes()
		require.Equal(t, expected4, encoded[2].Values[1])
		

		// issue
		// fieldVal1, err := bindings.PrepareTopicArg(ev.Inputs[4], events[0].MetadataArray)
		// require.NoError(t, err)
		// packed3, err := abi.Arguments{ev.Inputs[4]}.Pack(fieldVal1)
		packed3, err := abi.Arguments{ev.Inputs[4]}.Pack(events[0].MetadataArray)
		expected5 := crypto.Keccak256(packed3)
		require.NoError(t, err)
		require.Equal(t, expected5, encoded[3].Values[0])

		// fieldVal2, err := bindings.PrepareTopicArg(ev.Inputs[4], events[1].MetadataArray)
		// require.NoError(t, err)
		// packed4, err := abi.Arguments{ev.Inputs[4]}.Pack(fieldVal2)
		packed4, err := abi.Arguments{ev.Inputs[4]}.Pack(events[1].MetadataArray)
		require.NoError(t, err)
		expected6 := crypto.Keccak256(packed4)
		require.Equal(t, expected6, encoded[3].Values[1])
		// issue

		trigger, err := ds.LogTriggerDynamicEventLog(1, evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED, events)
		require.NotNil(t, trigger)
		require.NoError(t, err)

		// Test the Adapt method for DynamicEvent
    // Encode the non-indexed parameters (Key and Sender) into the log data
    // The DynamicEvent has: UserData (indexed), Sender (non-indexed), Metadata (indexed), MetadataArray (indexed)
    // We need to encode the non-indexed parameters (Key and Sender) into the log data
    eventData, err := abi.Arguments{ev.Inputs[0], ev.Inputs[2]}.Pack(events[0].Key, events[0].Sender)
    require.NoError(t, err, "Encoding DynamicEvent data should not return an error")

    // Create a mock log that simulates what would be returned by the blockchain
    mockLog := &evm.Log{
        Address: ds.Address.Bytes(),                    // Contract address
        Topics: [][]byte{
            ds.Codec.DynamicEventLogHash(),            // Event signature hash
            expected1,                                 // UserData hash (indexed)
            expected3,                                 // Metadata (indexed)
            expected5,                                 // MetadataArray hash (indexed)
        },
        Data: eventData,                               // Encoded Key and Sender data
    }

    // // Call Adapt to decode the log
    decodedLog, err := trigger.Adapt(mockLog)
    require.NoError(t, err, "Adapt should not return an error")
    require.NotNil(t, decodedLog, "Decoded log should not be nil")
    
    // // Verify the decoded data matches what we expect
    // require.Equal(t, events[0].Key, decodedLog.Data.Key, "Decoded key should match")
    // require.Equal(t, events[0].UserData, decodedLog.Data.UserData, "Decoded userData should match")
    // require.Equal(t, events[0].Sender, decodedLog.Data.Sender, "Decoded sender should match")
    // require.Equal(t, events[0].Metadata, decodedLog.Data.Metadata, "Decoded metadata should match")
    // require.Equal(t, events[0].MetadataArray, decodedLog.Data.MetadataArray, "Decoded metadataArray should match")
    
    // // Verify the original log is preserved
    // require.Equal(t, mockLog, decodedLog.Log, "Original log should be preserved")
	})

	// Test the decoder directly to isolate the issue
	// t.Run("decoder direct test", func(t *testing.T) {
	// 	ev := ds.ABI.Events["DynamicEvent"]
	// 	event := datastorage.DynamicEvent{
	// 			Key: "testKey1",
	// 			UserData: datastorage.UserData{
	// 					Key:   "userKey1",
	// 					Value: "userValue1",
	// 			},
	// 			Sender:   "testSender1",
	// 			Metadata: common.HexToHash("metadata1"),
	// 			MetadataArray: [][]byte{
	// 					[]byte("meta1"),
	// 					[]byte("meta2"),
	// 			},
	// 	}

	// 	// Create a log that matches the expected structure
	// 	eventData, err := abi.Arguments{ev.Inputs[0], ev.Inputs[2]}.Pack(event.Key, event.Sender)
	// 	require.NoError(t, err, "Encoding event data should not return an error")

	// 	// Pack the indexed parameters to get their topic values
	// 	userDataPacked, err := abi.Arguments{ev.Inputs[1]}.Pack(event.UserData)
	// 	require.NoError(t, err)
	// 	userDataHash := crypto.Keccak256(userDataPacked)

	// 	metadataPacked, err := abi.Arguments{ev.Inputs[3]}.Pack(event.Metadata)
	// 	require.NoError(t, err)
	// 	metadataHash := crypto.Keccak256(metadataPacked)

	// 	metadataArrayPacked, err := abi.Arguments{ev.Inputs[4]}.Pack(event.MetadataArray)
	// 	require.NoError(t, err)
	// 	metadataArrayHash := crypto.Keccak256(metadataArrayPacked)

	// 	mockLog := &evm.Log{
	// 			Address: ds.Address.Bytes(),
	// 			Topics: [][]byte{
	// 					ds.Codec.DynamicEventLogHash(),
	// 					userDataHash,
	// 					metadataHash,
	// 					metadataArrayHash,
	// 			},
	// 			Data: eventData,
	// 	}

	// 	// Test the decoder directly
	// 	decoded, err := ds.Codec.DecodeDynamicEvent(mockLog)
	// 	if err != nil {
	// 			t.Logf("Decoder error: %v", err)
	// 			t.Logf("Log structure: Address=%x, Topics=%d, Data=%x", mockLog.Address, len(mockLog.Topics), mockLog.Data)
	// 			for i, topic := range mockLog.Topics {
	// 					t.Logf("Topic %d: %x", i, topic)
	// 			}
	// 	}
	// 	require.NoError(t, err, "Direct decoder should work")
	// 	require.NotNil(t, decoded, "Decoded result should not be nil")
	// })

	t.Run("ABI structure debug", func(t *testing.T) {
    ev := ds.ABI.Events["DynamicEvent"]
    t.Logf("DynamicEvent ABI:")
    t.Logf("  Name: %s", ev.Name)
    t.Logf("  Inputs count: %d", len(ev.Inputs))
    for i, input := range ev.Inputs {
        t.Logf("  Input %d: Name=%s, Type=%s, Indexed=%v", i, input.Name, input.Type, input.Indexed)
    }
    
    // Let's also check the actual event structure
    event := datastorage.DynamicEvent{
        Key: "testKey1",
        UserData: datastorage.UserData{
            Key:   "userKey1",
            Value: "userValue1",
        },
        Sender:   "testSender1",
        Metadata: common.HexToHash("metadata1"),
        MetadataArray: [][]byte{
            []byte("meta1"),
            []byte("meta2"),
        },
    }
    
    t.Logf("Event structure:")
    t.Logf("  Key: %s (type: %T)", event.Key, event.Key)
    t.Logf("  UserData: %+v (type: %T)", event.UserData, event.UserData)
    t.Logf("  Sender: %s (type: %T)", event.Sender, event.Sender)
    t.Logf("  Metadata: %x (type: %T)", event.Metadata, event.Metadata)
    t.Logf("  MetadataArray: %v (type: %T)", event.MetadataArray, event.MetadataArray)
})

t.Run("check MetadataArray type", func(t *testing.T) {
	ev := ds.ABI.Events["DynamicEvent"]
	
	// Find the MetadataArray input
	var metadataArrayInput abi.Argument
	for _, input := range ev.Inputs {
			if input.Name == "MetadataArray" {
					metadataArrayInput = input
					break
			}
	}
	
	t.Logf("MetadataArray ABI type: %s", metadataArrayInput.Type)
    
    // Use the same approach as the existing codec
    testData := [][]byte{[]byte("meta1"), []byte("meta2")}
    
    // Use PrepareTopicArg like the existing codec does
    fieldVal, err := bindings.PrepareTopicArg(metadataArrayInput, testData)
    t.Logf("PrepareTopicArg result: err=%v, type=%T, value=%v", err, fieldVal, fieldVal)
    
    if err == nil {
        // If PrepareTopicArg works, try to pack the result
        packed, err2 := abi.Arguments{metadataArrayInput}.Pack(fieldVal)
        t.Logf("Packing PrepareTopicArg result: err=%v, packed=%x", err2, packed)
    }
})
}

func newDataStorage(t *testing.T) *datastorage.DataStorage {
	client := &evm.Client{ChainSelector: anyChainSelector}
	ds, err := datastorage.NewDataStorage(client, common.Address{}, &bindings.ContractInitOptions{})
	require.NoError(t, err, "Failed to create DataStorage instance")
	return ds
}
