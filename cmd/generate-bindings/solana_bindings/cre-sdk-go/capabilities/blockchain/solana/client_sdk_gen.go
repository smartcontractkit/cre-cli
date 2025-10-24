package solana

import (
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

type Client struct {
	ChainSelector uint64
	// TODO: https://smartcontract-it.atlassian.net/browse/CAPPL-799 allow defaults for capabilities
}

func (c *Client) ReadAccount(runtime cre.Runtime, input *ReadAccountRequest) cre.Promise[*ReadAccountReply] {
	return cre.PromiseFromResult[*ReadAccountReply](nil, nil)
}

func (c *Client) WriteReport(runtime cre.Runtime, input *WriteCreReportRequest) cre.Promise[*WriteReportReply] {
	return cre.PromiseFromResult[*WriteReportReply](nil, nil)
}

type WriteCreReportRequest struct {
	Receiver []byte

	Report *cre.Report
}

func LogTrigger(chainSelector uint64, config *FilterLogTriggerRequest) cre.Trigger[*Log, *Log] {
	return nil
}
