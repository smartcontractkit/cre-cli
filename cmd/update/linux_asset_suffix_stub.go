//go:build !linux

package update

func linuxAssetSuffix() string {
	return ""
}
