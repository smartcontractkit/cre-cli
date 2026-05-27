package common

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/viper"
)

// ResolveConfigPath returns the config path based on the --no-config,
// --config, and --default-config flag convention. defaultPath is the
// value from workflow.yaml settings.
func ResolveConfigPath(v *viper.Viper, defaultPath string) string {
	if v.GetBool("no-config") {
		return ""
	}
	if cfgFlag := v.GetString("config"); cfgFlag != "" {
		return cfgFlag
	}
	return defaultPath
}

// IsURL returns true when s begins with http:// or https://.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// FetchURL performs an HTTP GET and returns the response body bytes.
func FetchURL(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP GET %s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", url, err)
	}
	return data, nil
}
