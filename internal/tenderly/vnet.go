package tenderly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	EnvAccessKey    = "TENDERLY_ACCESS_KEY"
	EnvAccountSlug  = "TENDERLY_ACCOUNT_SLUG"
	EnvProjectSlug  = "TENDERLY_PROJECT_SLUG"
	tenderlyBaseURL = "https://api.tenderly.co/api/v1"
)

// networkIDMap maps CRE chain names to Tenderly network_id (chain ID).
var networkIDMap = map[string]int{
	// L1 chains
	"ethereum-mainnet":         1,
	"ethereum-testnet-sepolia": 11155111,
	"avalanche-mainnet":        43114,
	"avalanche-testnet-fuji":   43113,
	"bsc-mainnet":              56,
	"bsc-testnet":              97,
	"polygon-mainnet":          137,
	"polygon-testnet-amoy":     80002,
	"gnosis-mainnet":           100,
	"gnosis-testnet-chiado":    10200,

	// L2 chains (flat names)
	"arbitrum-mainnet":         42161,
	"arbitrum-testnet-sepolia": 421614,
	"base-mainnet":             8453,
	"base-sepolia":             84532,
	"optimism-mainnet":         10,
	"optimism-testnet-sepolia": 11155420,

	// L2 chains (nested CRE names: <l1>-<l2>-<index>)
	"ethereum-mainnet-arbitrum-1":           42161,
	"ethereum-mainnet-base-1":              8453,
	"ethereum-mainnet-optimism-1":          10,
	"ethereum-mainnet-zksync-1":            324,
	"ethereum-mainnet-worldchain-1":        480,
	"ethereum-mainnet-xlayer-1":            196,
	"ethereum-testnet-sepolia-arbitrum-1":  421614,
	"ethereum-testnet-sepolia-base-1":      84532,
	"ethereum-testnet-sepolia-optimism-1":  11155420,
	"ethereum-testnet-sepolia-zksync-1":    300,
	"ethereum-testnet-sepolia-worldchain-1": 4801,
	"ethereum-testnet-sepolia-linea-1":     59141,

	// Other
	"pharos-atlantic": 688688,
}

// VnetResult contains the RPC and dashboard URLs for each requested network.
type VnetResult struct {
	NetworkRPCs map[string]string // chain-name -> admin rpc URL (for project config)
	PublicRPCs  map[string]string // chain-name -> public rpc URL (shareable, no auth needed)
	VnetURLs    map[string]string // chain-name -> Tenderly dashboard URL
}

// Provider creates Tenderly Virtual Networks for the given chains.
type Provider interface {
	CreateVnets(networks []string) (*VnetResult, error)
}

// --- API types ---

type createVnetRequest struct {
	Slug               string              `json:"slug"`
	DisplayName        string              `json:"display_name"`
	ForkConfig         forkConfig          `json:"fork_config"`
	VirtualNetworkConf virtualNetworkConf  `json:"virtual_network_config"`
	SyncStateConfig    syncStateConfig     `json:"sync_state_config"`
	ExplorerPageConfig explorerPageConfig  `json:"explorer_page_config"`
}

type forkConfig struct {
	NetworkID   int    `json:"network_id"`
	BlockNumber string `json:"block_number"`
}

type virtualNetworkConf struct {
	ChainConfig chainConfig `json:"chain_config"`
}

type chainConfig struct {
	ChainID int `json:"chain_id"`
}

type syncStateConfig struct {
	Enabled         bool   `json:"enabled"`
	CommitmentLevel string `json:"commitment_level"`
}

type explorerPageConfig struct {
	Enabled                bool   `json:"enabled"`
	VerificationVisibility string `json:"verification_visibility"`
}

type createVnetResponse struct {
	ID   string       `json:"id"`
	RPCs []rpcEntry   `json:"rpcs"`
}

type rpcEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// --- APIProvider ---

// APIProvider creates Tenderly Virtual TestNets via the REST API.
type APIProvider struct {
	accessKey   string
	accountSlug string
	projectSlug string
	userID      string // Auth0 user ID, prefixed to vnet slug for per-user isolation
	httpClient  *http.Client
	baseURL     string
}

// NewAPIProvider reads credentials from environment variables and returns a provider.
// userID is the Auth0 user ID used to namespace VNets per user.
func NewAPIProvider(userID string) (*APIProvider, error) {
	accessKey := os.Getenv(EnvAccessKey)
	if accessKey == "" {
		return nil, fmt.Errorf(
			"environment variable %s is not set\n\n"+
				"To use Tenderly Virtual TestNets, set your API credentials:\n"+
				"  export %s=<your-tenderly-api-key>\n"+
				"  export %s=<your-account-slug>\n"+
				"  export %s=<your-project-slug>\n\n"+
				"You can find these at https://dashboard.tenderly.co/account/authorization",
			EnvAccessKey, EnvAccessKey, EnvAccountSlug, EnvProjectSlug,
		)
	}

	accountSlug := os.Getenv(EnvAccountSlug)
	if accountSlug == "" {
		return nil, fmt.Errorf("environment variable %s is not set", EnvAccountSlug)
	}

	projectSlug := os.Getenv(EnvProjectSlug)
	if projectSlug == "" {
		return nil, fmt.Errorf("environment variable %s is not set", EnvProjectSlug)
	}

	if userID == "" {
		return nil, fmt.Errorf("user ID is required to create per-user Tenderly Virtual TestNets (run cre login first)")
	}

	return &APIProvider{
		accessKey:   accessKey,
		accountSlug: accountSlug,
		projectSlug: projectSlug,
		userID:      userID,
		httpClient:  &http.Client{},
		baseURL:     tenderlyBaseURL,
	}, nil
}

// CreateVnets creates one Virtual TestNet per network and returns the RPC URLs.
func (p *APIProvider) CreateVnets(networks []string) (*VnetResult, error) {
	result := &VnetResult{
		NetworkRPCs: make(map[string]string, len(networks)),
		PublicRPCs:  make(map[string]string, len(networks)),
		VnetURLs:    make(map[string]string, len(networks)),
	}

	for _, network := range networks {
		netID, ok := networkIDMap[network]
		if !ok {
			return nil, fmt.Errorf("unsupported network %q for Tenderly Virtual TestNets", network)
		}

		vnet, err := p.createVnet(network, netID)
		if err != nil {
			return nil, fmt.Errorf("failed to create vnet for %s: %w", network, err)
		}

		result.NetworkRPCs[network] = vnet.adminRPC
		result.PublicRPCs[network] = vnet.publicRPC
		result.VnetURLs[network] = vnet.dashboardURL
	}

	return result, nil
}

type vnetURLs struct {
	adminRPC     string
	publicRPC    string
	dashboardURL string
}

func (p *APIProvider) createVnet(network string, netID int) (*vnetURLs, error) {
	// Sanitize user ID for use in slug (Auth0 IDs contain "|", e.g. "auth0|abc123")
	sanitizedUID := strings.ReplaceAll(p.userID, "|", "-")
	slug := fmt.Sprintf("cre-%s-%s", sanitizedUID, strings.ReplaceAll(network, "/", "-"))

	reqBody := createVnetRequest{
		Slug:        slug,
		DisplayName: fmt.Sprintf("CRE %s", network),
		ForkConfig: forkConfig{
			NetworkID:   netID,
			BlockNumber: "latest",
		},
		VirtualNetworkConf: virtualNetworkConf{
			ChainConfig: chainConfig{
				ChainID: netID,
			},
		},
		SyncStateConfig: syncStateConfig{
			Enabled: false,
		},
		ExplorerPageConfig: explorerPageConfig{
			Enabled:                true,
			VerificationVisibility: "bytecode",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/account/%s/project/%s/vnets", p.baseURL, p.accountSlug, p.projectSlug)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Access-Key", p.accessKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var vnetResp createVnetResponse
	if err := json.Unmarshal(respBody, &vnetResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var adminRPC, publicRPC string
	for _, rpc := range vnetResp.RPCs {
		switch rpc.Name {
		case "Admin RPC":
			adminRPC = rpc.URL
		case "Public RPC":
			publicRPC = rpc.URL
		}
	}
	// Use admin RPC for project config; fall back to first available
	if adminRPC == "" && len(vnetResp.RPCs) > 0 {
		adminRPC = vnetResp.RPCs[0].URL
	}
	if adminRPC == "" {
		return nil, fmt.Errorf("no RPC URL in response for %s", network)
	}

	return &vnetURLs{
		adminRPC:  adminRPC,
		publicRPC: publicRPC,
		dashboardURL: fmt.Sprintf("https://dashboard.tenderly.co/%s/%s/testnet/%s",
			p.accountSlug, p.projectSlug, vnetResp.ID),
	}, nil
}

// NetworkID returns the Tenderly network ID for a CRE chain name, or 0 if unsupported.
func NetworkID(network string) (int, bool) {
	id, ok := networkIDMap[network]
	return id, ok
}
