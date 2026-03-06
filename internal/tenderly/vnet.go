package tenderly

import (
	"fmt"
	"os"
)

const EnvVnetURL = "CRE_TENDERLY_VNET_URL"

// VnetResult contains the RPC and debug URLs for each requested network.
type VnetResult struct {
	NetworkRPCs map[string]string // chain-name -> rpc URL
	VnetURLs    map[string]string // chain-name -> debug URL (same as RPC for POC)
}

// Provider creates Tenderly Virtual Networks for the given chains.
type Provider interface {
	CreateVnets(networks []string) (*VnetResult, error)
}

// EnvProvider reads CRE_TENDERLY_VNET_URL from the environment and returns it
// as the RPC URL for all requested chains. This is a POC implementation that
// will later be replaced with Tenderly API calls.
type EnvProvider struct{}

// NewEnvProvider returns a new EnvProvider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

func (p *EnvProvider) CreateVnets(networks []string) (*VnetResult, error) {
	baseURL := os.Getenv(EnvVnetURL)
	if baseURL == "" {
		return nil, fmt.Errorf(
			"environment variable %s is not set\n\n"+
				"To use Tenderly Virtual Networks, set the vnet base URL:\n"+
				"  export %s=https://rpc.tenderly.co/vnet/your-vnet-id\n\n"+
				"You can create a virtual network at https://dashboard.tenderly.co",
			EnvVnetURL, EnvVnetURL,
		)
	}

	result := &VnetResult{
		NetworkRPCs: make(map[string]string, len(networks)),
		VnetURLs:    make(map[string]string, len(networks)),
	}

	for _, network := range networks {
		result.NetworkRPCs[network] = baseURL
		result.VnetURLs[network] = baseURL
	}

	return result, nil
}
