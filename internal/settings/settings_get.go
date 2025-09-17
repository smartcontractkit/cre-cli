package settings

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"

	chainSelectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/ethkeys"
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
	ChainSelector uint64 `mapstructure:"chain-selector" yaml:"chain-selector"`
	// TODO: in the future, we can have a distinction between "public URL" and "private URL", with only one of them present at the time
	// "public URL" would be URL hidden behind the VPN or URL from ChainList, something that doesn't contain sensitive API tokens, e.g.
	// url_public: https://rpcs.cldev.sh/ethereum/sepolia
	// "private URL" can be feeded to the settings file by specifying the env var name where the real URL is kept, e.g.
	// url_private: RPC_URL_ETH_SEPOLIA
	Url string `mapstructure:"url" yaml:"url"`
}

func GetRpcUrlSettings(v *viper.Viper, chainSelector uint64) (string, error) {
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
		if rpc.ChainSelector == chainSelector {
			return rpc.Url, nil
		}
	}

	return "", fmt.Errorf("rpc url not found for chain %d", chainSelector)
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

	if v.IsSet(Flags.RawTxFlag.Name) {
		return v.GetString(fmt.Sprintf("%s.%s", target, WorkflowOwnerSettingName)), constants.WorkflowOwnerTypeMSIG, nil
	}

	if v.IsSet(Flags.Owner.Name) {
		ownerFlag := v.GetString(Flags.Owner.Name)
		if ownerFlag != "" {
			v.Set(fmt.Sprintf("%s.%s", target, WorkflowOwnerSettingName), ownerFlag)
		}
		return ownerFlag, constants.WorkflowOwnerTypeMSIG, nil
	}

	rawPrivKey := v.GetString(EthPrivateKeyEnvVar)
	normPrivKey := NormalizeHexKey(rawPrivKey)
	ownerAddress, err = ethkeys.DeriveEthAddressFromPrivateKey(normPrivKey)
	if err != nil {
		return "", "", err
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

	return "", fmt.Errorf(
		"target not set: specify --%s or set %s env var",
		Flags.Target.Name, CreTargetEnvVar,
	)
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
