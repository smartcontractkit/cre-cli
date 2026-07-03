//go:build windows

package update

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// windowsSignerSubject is a substring of the Authenticode certificate subject.
const windowsSignerSubject = "Smart Contract"

func verifyReleaseBinary(binPath, _ string) error {
	escaped := strings.ReplaceAll(binPath, "'", "''")
	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'; $s = Get-AuthenticodeSignature -FilePath '%s'; if ($s.Status -ne 'Valid') { Write-Error ('authenticode status: ' + $s.Status); exit 1 }; if (-not $s.SignerCertificate) { Write-Error 'missing signer certificate'; exit 1 }; if ($s.SignerCertificate.Subject -notlike '*%s*') { Write-Error ('unexpected signer: ' + $s.SignerCertificate.Subject); exit 2 }`,
		escaped,
		windowsSignerSubject,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script) // #nosec G204 -- script uses escaped path only
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("authenticode verification failed: %s: %w", msg, err)
		}
		return fmt.Errorf("authenticode verification failed: %w", err)
	}
	return nil
}
