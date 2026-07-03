package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/smartcontractkit/cre-cli/install"
	"github.com/stretchr/testify/require"
)

func TestReleasePublicKeyMatchesInstall(t *testing.T) {
	embedded := install.ReleasePublicKey
	require.NotEmpty(t, embedded)

	canonical, err := os.ReadFile(filepath.Join("public_key.asc"))
	require.NoError(t, err)

	require.Equal(t, canonical, embedded, "embedded public_key.asc must match install/public_key.asc (symlink target)")
}

func TestGetSigAssetName(t *testing.T) {
	require.Equal(t, "cre_linux_amd64.sig", getSigAssetName("linux", "amd64"))
	require.Equal(t, "cre_darwin_arm64.sig", getSigAssetName("darwin", "arm64"))
}

func TestGetAssetName(t *testing.T) {
	asset, platform, archName, err := getAssetName()
	require.NoError(t, err)
	require.NotEmpty(t, asset)
	require.NotEmpty(t, platform)
	require.NotEmpty(t, archName)
	require.Contains(t, asset, "cre_"+platform+"_"+archName)
}
