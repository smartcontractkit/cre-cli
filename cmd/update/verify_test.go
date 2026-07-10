package update

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSigAssetName(t *testing.T) {
	require.Equal(t, "cre_linux_amd64.sig", getSigAssetName("linux", "amd64", ""))
	require.Equal(t, "cre_linux_amd64_ldd2-35.sig", getSigAssetName("linux", "amd64", "_ldd2-35"))
}

func TestGetAssetName(t *testing.T) {
	asset, platform, archName, linuxSuffix, err := getAssetName()
	require.NoError(t, err)
	require.NotEmpty(t, asset)
	require.NotEmpty(t, platform)
	require.NotEmpty(t, archName)
	require.Contains(t, asset, "cre_"+platform+"_"+archName+linuxSuffix)
}
