package capabilitiesregistry

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
)

const (
	defaultPageSize = 32
	maxNodesPerPage = 256
)

// Client is a read-only CapabilitiesRegistry contract client backed by a validated RPC URL.
type Client struct {
	contract *capreg.CapabilitiesRegistry
	eth      *ethclient.Client
}

// NewReadOnlyClient dials rpcURL and binds contractAddress. The caller must have already
// validated rpcURL format and chain ID against the tenant chain selector.
func NewReadOnlyClient(ctx context.Context, rpcURL, contractAddress string) (*Client, error) {
	if !common.IsHexAddress(contractAddress) {
		return nil, fmt.Errorf("invalid capabilities registry address %q", contractAddress)
	}

	backend, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial capabilities registry RPC: %w", err)
	}

	addr := common.HexToAddress(contractAddress)
	contract, err := capreg.NewCapabilitiesRegistry(addr, backend)
	if err != nil {
		backend.Close()
		return nil, fmt.Errorf("bind capabilities registry at %s: %w", contractAddress, err)
	}

	return &Client{contract: contract, eth: backend}, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	if c == nil || c.eth == nil {
		return
	}
	c.eth.Close()
	c.eth = nil
}

// GetDONsInFamily returns all DON IDs registered under donFamily.
func (c *Client) GetDONsInFamily(ctx context.Context, donFamily string) ([]*big.Int, error) {
	callOpts := &bind.CallOpts{Context: ctx}
	start := big.NewInt(0)
	limit := big.NewInt(defaultPageSize)

	var all []*big.Int
	for {
		batch, err := c.contract.GetDONsInFamily(callOpts, donFamily, start, limit)
		if err != nil {
			return nil, fmt.Errorf("GetDONsInFamily(%q): %w", donFamily, err)
		}
		all = append(all, batch...)
		if len(batch) < int(limit.Int64()) {
			break
		}
		start.Add(start, limit)
	}
	return all, nil
}

// GetDON returns on-chain metadata for a single DON ID.
func (c *Client) GetDON(ctx context.Context, donID uint32) (capreg.CapabilitiesRegistryDONInfo, error) {
	callOpts := &bind.CallOpts{Context: ctx}
	don, err := c.contract.GetDON(callOpts, donID)
	if err != nil {
		return capreg.CapabilitiesRegistryDONInfo{}, fmt.Errorf("GetDON(%d): %w", donID, err)
	}
	return don, nil
}

// GetNodes returns all nodes registered in the CapabilitiesRegistry contract.
func (c *Client) GetNodes(ctx context.Context) ([]capreg.INodeInfoProviderNodeInfo, error) {
	callOpts := &bind.CallOpts{Context: ctx}
	start := big.NewInt(0)
	limit := big.NewInt(maxNodesPerPage)

	var all []capreg.INodeInfoProviderNodeInfo
	for {
		batch, err := c.contract.GetNodes(callOpts, start, limit)
		if err != nil {
			return nil, fmt.Errorf("GetNodes: %w", err)
		}
		all = append(all, batch...)
		if len(batch) < int(limit.Int64()) {
			break
		}
		start.Add(start, limit)
	}
	return all, nil
}
