package test

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
)

const (
	SethConfigPath    = "seth.toml"
	TestChainSelector = uint64(7759470850252068959)
	SettingsTarget    = "production-testnet"
)

// strip the ANSI escape codes from the output
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var CLIPath = os.TempDir() + string(os.PathSeparator) + "cre" + func() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}()

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// Use viper to anchor cre config file defaults, override them where necessary
// As a result, write the temporary config file only needed for tests
func createCliSettingsFile(
	testConfig *TestConfig,
	workflowOwner string,
	workflowName string,
	testEthURL string,
) error {
	trimmedName := strings.TrimSpace(workflowName)
	if len(trimmedName) < 10 {
		return fmt.Errorf("workflow name %q is too short, minimum length is 10 characters", trimmedName)
	}

	v := viper.New()

	v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.DONFamilySettingName), constants.DefaultStagingDonFamily)

	// user-workflow fields
	if workflowOwner != "" {
		v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.WorkflowOwnerSettingName), workflowOwner)
	}
	v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.WorkflowNameSettingName), trimmedName)

	// rpcs
	v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.RpcsSettingName), []settings.RpcEndpoint{
		{
			ChainSelector: TestChainSelector,
			Url:           testEthURL,
		},
	})

	// write YAML
	v.SetConfigType("yaml")
	if err := v.WriteConfigAs(testConfig.WorkflowSettingsFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	L.Debug().
		Str("WorkflowSettingsFile", testConfig.WorkflowSettingsFile).
		Interface("Config", v.AllSettings()).
		Msg("Config file created")

	return nil
}

func createCliEnvFile(envPath string, ethPrivateKey string) error {
	file, err := os.OpenFile(envPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}
	_, err = writer.WriteString(fmt.Sprintf("%s=%s", settings.EthPrivateKeyEnvVar, ethPrivateKey))
	if err != nil {
		return err
	}

	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}

	_, err = writer.WriteString(fmt.Sprintf("%s=%s", settings.CreTargetEnvVar, SettingsTarget))
	if err != nil {
		return err
	}

	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}
	writer.Flush()

	return nil
}

func initTestEnv(t *testing.T) (*os.Process, string) {
	InitLogging()
	anvilProc, anvilPort, err := StartAnvil(LOAD_ANVIL_STATE)
	require.NoError(t, err, "Failed to start Anvil")
	ethUrl := "http://localhost:" + strconv.Itoa(anvilPort)
	return anvilProc, ethUrl
}
