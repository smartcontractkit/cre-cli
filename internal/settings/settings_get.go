package settings

import (
	"errors"
	"fmt"
	"os"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	chainSelectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
)

type ContractGroups struct {
	Registries []Contract `mapstructure:"registries" yaml:"registries"`
	KeyStone   []Contract `mapstructure:"keystone" yaml:"keystone"`
}

type Contract struct {
	Name          string `mapstructure:"name" yaml:"name"`
	Address       string `mapstructure:"address" yaml:"address"`
	ChainSelector uint64 `mapstructure:"chain-selector" yaml:"chain-selector"`
}

type RpcEndpoint struct {
	ChainName string `mapstructure:"chain-name" yaml:"chain-name"`
	// TODO: in the future, we can have a distinction between "public URL" and "private URL", with only one of them present at the time
	// "public URL" would be URL hidden behind the VPN or URL from ChainList, something that doesn't contain sensitive API tokens, e.g.
	// url_public: https://rpcs.cldev.sh/ethereum/sepolia
	// "private URL" can be feeded to the settings file by specifying the env var name where the real URL is kept, e.g.
	// url_private: RPC_URL_ETH_SEPOLIA
	Url string `mapstructure:"url" yaml:"url"`
}

// ExperimentalChain represents an EVM chain not in official chain-selectors.
// Automatically used by the simulator when present in the target's experimental-chains config.
// The ChainSelector is used as the selector key for EVM clients and forwarders.
type ExperimentalChain struct {
	ChainSelector uint64 `mapstructure:"chain-selector" yaml:"chain-selector"`
	RPCURL        string `mapstructure:"rpc-url" yaml:"rpc-url"`
	Forwarder     string `mapstructure:"forwarder" yaml:"forwarder"`
}

func GetRpcUrlSettings(v *viper.Viper, chainName string) (string, error) {
	target, err := GetTarget(v)
	if err != nil {
		return "", err
	}

	keyWithTarget := fmt.Sprintf("%s.%s", target, RpcsSettingName)
	var rpcs []RpcEndpoint
	err = v.UnmarshalKey(keyWithTarget, &rpcs)
	if err != nil {
		return "", fmt.Errorf("not possible to unmarshall rpcs: %w", err)
	}

	for _, rpc := range rpcs {
		if rpc.ChainName == chainName {
			return rpc.Url, nil
		}
	}

	return "", fmt.Errorf("rpc url not found for chain %s", chainName)
}

// GetExperimentalChains reads the experimental-chains list from the current target.
// Returns an empty slice if the key is not set or unmarshalling fails.
func GetExperimentalChains(v *viper.Viper) ([]ExperimentalChain, error) {
	target, err := GetTarget(v)
	if err != nil {
		return nil, err
	}

	keyWithTarget := fmt.Sprintf("%s.%s", target, ExperimentalChainsSettingName)
	if !v.IsSet(keyWithTarget) {
		return nil, nil
	}

	var chains []ExperimentalChain
	err = v.UnmarshalKey(keyWithTarget, &chains)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal experimental-chains: %w", err)
	}

	return chains, nil
}

func GetEnvironmentVariable(filePath, key string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("key %s not found in %s", key, filePath)
}

func GetWorkflowOwner(v *viper.Viper) (ownerAddress string, ownerType string, err error) {
	target, err := GetTarget(v)
	if err != nil {
		return "", "", err
	}

	// if --unsigned flag or --changeset is set, owner must be set in settings
	ownerKey := fmt.Sprintf("%s.%s", target, WorkflowOwnerSettingName)
	if v.IsSet(Flags.RawTxFlag.Name) || v.IsSet(Flags.Changeset.Name) {
		if v.IsSet(ownerKey) {
			owner := strings.TrimSpace(v.GetString(ownerKey))
			if owner != "" {
				return owner, constants.WorkflowOwnerTypeMSIG, nil
			}
		}

		// Not set or empty -> print error and stop
		msg := fmt.Sprintf(
			"missing workflow owner: when using --%s you must set %q in your config",
			Flags.RawTxFlag.Name, ownerKey,
		)
		fmt.Fprintln(os.Stderr, msg)
		return "", "", errors.New(msg)
	}

	// unsigned or changeset is not set, it is EOA path
	rawPrivKey := v.GetString(EthPrivateKeyEnvVar)
	normPrivKey := NormalizeHexKey(rawPrivKey)
	ownerAddress, err = ethkeys.DeriveEthAddressFromPrivateKey(normPrivKey)
	if err != nil {
		return "", "", err
	}

	// if owner is also set in settings, owner and private key should match
	if v.IsSet(ownerKey) {
		cfgOwner := strings.TrimSpace(v.GetString(ownerKey))
		if cfgOwner != "" {
			// Validate cfgOwner and compare to derived ownerAddress
			derived := ethcommon.HexToAddress(ownerAddress)
			fromCfg := ethcommon.HexToAddress(cfgOwner)
			if derived != fromCfg {
				return "", "", fmt.Errorf(
					"settings owner %q does not match address derived from private key %q. "+
						"remove owner in settings if you are using EOA",
					fromCfg.Hex(), derived.Hex(),
				)
			}
		}
	}

	return ownerAddress, constants.WorkflowOwnerTypeEOA, nil
}

func GetTarget(v *viper.Viper) (string, error) {
	if v.IsSet(Flags.Target.Name) {
		target := v.GetString(Flags.Target.Name)
		if target != "" {
			return target, nil
		}
	}

	target := v.GetString(CreTargetEnvVar)
	if target != "" {
		return target, nil
	}

	return "", nil
}

// GetAvailableTargets reads project.yaml and returns the top-level keys
// that represent target configurations, preserving the order from the file.
func GetAvailableTargets() ([]string, error) {
	projectPath, err := getProjectSettingsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find project settings: %w", err)
	}

	data, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project settings: %w", err)
	}

	// Parse with yaml.v3 Node to preserve key order
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse project settings: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}

	// Mapping nodes alternate key, value, key, value...
	// Only include keys whose values are mappings (actual target configs).
	var targets []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Kind == yaml.ScalarNode && val.Kind == yaml.MappingNode {
			targets = append(targets, key.Value)
		}
	}

	return targets, nil
}

func GetChainNameByChainSelector(chainSelector uint64) (string, error) {
	chainFamily, err := chainSelectors.GetSelectorFamily(chainSelector)
	if err != nil {
		return "", err
	}

	chainID, err := chainSelectors.GetChainIDFromSelector(chainSelector)
	if err != nil {
		return "", err
	}

	chainDetails, err := chainSelectors.GetChainDetailsByChainIDAndFamily(chainID, chainFamily)
	if err != nil {
		return "", err
	}

	return chainDetails.ChainName, nil
}

func GetChainSelectorByChainName(name string) (uint64, error) {
	chainID, err := chainSelectors.ChainIdFromName(name)
	if err != nil {
		return 0, fmt.Errorf("failed to get chain ID from name %q: %w", name, err)
	}

	selector, err := chainSelectors.SelectorFromChainId(chainID)
	if err != nil {
		return 0, fmt.Errorf("failed to get selector from chain ID %d: %w", chainID, err)
	}

	return selector, nil
}
