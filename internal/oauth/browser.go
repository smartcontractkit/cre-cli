package oauth

import (
	"fmt"
	"os/exec"
)

// OpenBrowser opens urlStr in the default browser for the given GOOS value.
func OpenBrowser(urlStr string, goos string) error {
	switch goos {
	case "darwin":
		return exec.Command("open", urlStr).Start()
	case "linux":
		return exec.Command("xdg-open", urlStr).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", goos)
	}
}
