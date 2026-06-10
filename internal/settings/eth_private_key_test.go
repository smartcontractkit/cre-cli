package settings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestEthPrivateKeyHex_RedactedInStringer(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", settings.EthPrivateKeyHex("").String())
	assert.Equal(t, "[REDACTED]", settings.EthPrivateKeyHex("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80").String())
	assert.Equal(t, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		settings.EthPrivateKeyHex("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80").Hex())
}

func TestDefaultEthPrivateKeyEnvPlaceholderUsedInInitTemplate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, settings.DefaultEthPrivateKeyEnvPlaceholder, "your-eth-private-key")
}
