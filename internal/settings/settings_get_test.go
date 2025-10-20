package settings_test

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestGetWorkflowOwner(t *testing.T) {
	validPrivKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	expectedOwner := "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	t.Run("derives owner from eth private key", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "test")
		v.Set(settings.EthPrivateKeyEnvVar, validPrivKey)

		owner, ownerType, err := settings.GetWorkflowOwner(v)
		assert.NoError(t, err)
		assert.Equal(t, expectedOwner, owner)
		assert.Equal(t, constants.WorkflowOwnerTypeEOA, ownerType)
	})

	t.Run("returns error for invalid eth private key", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "test")
		v.Set(settings.EthPrivateKeyEnvVar, "invalid")

		owner, ownerType, err := settings.GetWorkflowOwner(v)
		assert.Error(t, err)
		assert.Equal(t, "", owner)
		assert.Equal(t, "", ownerType)
	})
}

func TestGetTarget_FlagOverridesEnv(t *testing.T) {
	v := viper.New()
	v.Set(settings.Flags.Target.Name, "flagTarget")
	v.Set(settings.CreTargetEnvVar, "envTarget")

	got, err := settings.GetTarget(v)
	assert.NoError(t, err)
	assert.Equal(t, "flagTarget", got)
}

func TestGetTarget_EnvWhenNoFlag(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "envOnly")

	got, err := settings.GetTarget(v)
	assert.NoError(t, err)
	assert.Equal(t, "envOnly", got)
}

func TestGetTarget_ErrorWhenNeither(t *testing.T) {
	v := viper.New()

	_, err := settings.GetTarget(v)
	assert.Error(t, err)
}
