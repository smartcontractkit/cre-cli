package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDeployService(t *testing.T) {
	t.Parallel()

	t.Run("returns onchain service for onchain target", func(t *testing.T) {
		t.Parallel()
		target := registryTarget{targetType: registryTargetOnchain}
		svc := newDeployService(target, &handler{})
		_, ok := svc.(*onchainDeployService)
		assert.True(t, ok, "expected onchainDeployService for onchain target")
	})

	t.Run("returns private service for private target", func(t *testing.T) {
		t.Parallel()
		target := registryTarget{targetType: registryTargetPrivate}
		svc := newDeployService(target, &handler{})
		_, ok := svc.(*privateDeployService)
		assert.True(t, ok, "expected privateDeployService for private target")
	})
}
