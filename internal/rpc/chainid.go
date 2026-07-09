package rpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	gethrpc "github.com/ethereum/go-ethereum/rpc"

	chainSelectors "github.com/smartcontractkit/chain-selectors"
)

// QueryEthChainID dials rpcURL and returns the chain ID from eth_chainId.
func QueryEthChainID(rpcURL string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	client, err := gethrpc.DialContext(ctx, rpcURL)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	var chainIDHex string
	if err := client.CallContext(ctx, &chainIDHex, "eth_chainId"); err != nil {
		return 0, err
	}

	return strconv.ParseUint(chainIDHex, 0, 64)
}

// ValidateMatchesSelector verifies the RPC's eth_chainId matches expectedSelector.
func ValidateMatchesSelector(rpcURL string, expectedSelector uint64) error {
	rpcChainID, err := QueryEthChainID(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to verify RPC chain ID: %w", err)
	}

	expectedChainIDRaw, err := chainSelectors.GetChainIDFromSelector(expectedSelector)
	if err != nil {
		return fmt.Errorf("invalid chain selector %d: %w", expectedSelector, err)
	}
	expectedChainID, err := strconv.ParseUint(expectedChainIDRaw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chain ID %q for selector %d: %w", expectedChainIDRaw, expectedSelector, err)
	}

	if rpcChainID != expectedChainID {
		return fmt.Errorf(
			"RPC URL points to chain ID %d, but expected chain ID %d (selector %d); check your project RPC settings",
			rpcChainID, expectedChainID, expectedSelector,
		)
	}

	return nil
}
