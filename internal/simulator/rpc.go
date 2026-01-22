package simulator

import (
	"fmt"
	"net/rpc"
)

const (
	DefaultPort = 1337
)

type RunArgs struct {
	WorkflowID     string
	WorkflowOwner  string
	WorkflowName   string
	WorkflowConfig string

	BinaryPath string
}

type RpcClient struct {
	client *rpc.Client
}

func NewRpcClient(addr string, port int) (*RpcClient, error) {
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}
	return &RpcClient{
		client: client,
	}, nil
}

func (c *RpcClient) Run(args RunArgs) (int, error) {
	var reply int
	if err := c.client.Call("RpcHandler.Run", &args, &reply); err != nil {
		return -1, err
	}
	return reply, nil
}

func (c *RpcClient) Close() error {
	c.client.Close()
	return nil
}
