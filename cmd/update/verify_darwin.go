//go:build darwin

package update

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func verifyReleaseBinary(binPath, _ string) error {
	cmd := exec.Command("codesign", "--verify", "--strict", "--identifier", codesignIdentifier, binPath) // #nosec G204 -- fixed args, user-controlled path only
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("codesign verification failed: %s: %w", msg, err)
		}
		return fmt.Errorf("codesign verification failed: %w", err)
	}
	return nil
}
