package solana

import (
	"errors"
	"strconv"

	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"google.golang.org/protobuf/types/known/anypb"
)

type Client struct {
	ChainSelector uint64
	// TODO: https://smartcontract-it.atlassian.net/browse/CAPPL-799 allow defaults for capabilities
}

func (c *Client) GetAccountInfoWithOpts(runtime cre.Runtime, req *GetAccountInfoRequest) cre.Promise[*GetAccountInfoReply] {
	wrapped := &anypb.Any{}

	capCallResponse := cre.Then(runtime.CallCapability(&sdkpb.CapabilityRequest{
		Id:      "solana" + ":ChainSelector:" + strconv.FormatUint(c.ChainSelector, 10) + "@1.0.0",
		Payload: wrapped,
		Method:  "GetAccountInfoWithOpts",
	}), func(i *sdkpb.CapabilityResponse) (*GetAccountInfoReply, error) {
		switch payload := i.Response.(type) {
		case *sdkpb.CapabilityResponse_Error:
			return nil, errors.New(payload.Error)
		case *sdkpb.CapabilityResponse_Payload:
			output := &GetAccountInfoReply{}
			err := payload.Payload.UnmarshalTo(output)
			return output, err
		default:
			return nil, errors.New("unexpected response type")
		}
	})

	return capCallResponse
}

func (c *Client) GetMultipleAccountsWithOpts(runtime cre.Runtime, req GetMultipleAccountsRequest) cre.Promise[*GetMultipleAccountsReply] {
	return cre.PromiseFromResult[*GetMultipleAccountsReply](nil, nil)
}

func (c *Client) SimulateTX(runtime cre.Runtime, input *SimulateTXRequest) cre.Promise[*SimulateTXReply] {
	return cre.PromiseFromResult[*SimulateTXReply](nil, nil)
}

func (c *Client) WriteReport(runtime cre.Runtime, input *WriteCreReportRequest) cre.Promise[*WriteReportReply] {
	wrapped := &anypb.Any{}

	capCallResponse := cre.Then(runtime.CallCapability(&sdkpb.CapabilityRequest{
		Id:      "solana" + ":ChainSelector:" + strconv.FormatUint(c.ChainSelector, 10) + "@1.0.0",
		Payload: wrapped,
		Method:  "WriteReport",
	}), func(i *sdkpb.CapabilityResponse) (*WriteReportReply, error) {
		switch payload := i.Response.(type) {
		case *sdkpb.CapabilityResponse_Error:
			return nil, errors.New(payload.Error)
		case *sdkpb.CapabilityResponse_Payload:
			output := &WriteReportReply{}
			err := payload.Payload.UnmarshalTo(output)
			return output, err
		default:
			return nil, errors.New("unexpected response type")
		}
	})

	return capCallResponse
}

func LogTrigger(chainSelector uint64, config *FilterLogTriggerRequest) cre.Trigger[*Log, *Log] {
	return nil
}
