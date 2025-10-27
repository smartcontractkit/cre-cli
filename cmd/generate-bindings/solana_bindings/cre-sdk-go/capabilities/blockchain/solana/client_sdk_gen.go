package solana

import (
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

type Client struct {
	ChainSelector uint64
	// TODO: https://smartcontract-it.atlassian.net/browse/CAPPL-799 allow defaults for capabilities
}

func (c *Client) GetAccountInfoWithOpts(runtime cre.Runtime, req GetAccountInfoRequest) cre.Promise[*GetAccountInfoReply] {
	return cre.PromiseFromResult[*GetAccountInfoReply](nil, nil)
}

func (c *Client) GetMultipleAccountsWithOpts(runtime cre.Runtime, req GetMultipleAccountsRequest) cre.Promise[*GetMultipleAccountsReply] {
	return cre.PromiseFromResult[*GetMultipleAccountsReply](nil, nil)
}

func (c *Client) SimulateTX(runtime cre.Runtime, input *SimulateTXRequest) cre.Promise[*SimulateTXReply] {
	return cre.PromiseFromResult[*SimulateTXReply](nil, nil)
}

func (c *Client) WriteReport(runtime cre.Runtime, input *WriteCreReportRequest) cre.Promise[*WriteReportReply] {
	return cre.PromiseFromResult[*WriteReportReply](nil, nil)
}

func LogTrigger(chainSelector uint64, config *FilterLogTriggerRequest) cre.Trigger[*Log, *Log] {
	return nil
}
