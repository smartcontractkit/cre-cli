package update

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	osruntime "runtime"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const backupSuffix = ".bak"

func verifyBinaryRuns(binPath string) error {
	cmd := exec.Command(binPath, "version") // #nosec G204 -- binPath is a verified release artifact
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

func replaceSelf(newBin string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return replaceBinaryAt(self, newBin)
}

func replaceBinaryAt(self, newBin string) error {
	if osruntime.GOOS == "windows" {
		ui.Warning("Automatic replacement not supported on Windows")
		ui.Dim("Please close all running cre processes and manually replace the binary at:")
		ui.Code(self)
		ui.Dim("New binary downloaded at:")
		ui.Code(newBin)
		return fmt.Errorf("automatic replacement not supported on Windows")
	}

	backupPath := self + backupSuffix
	_ = os.Remove(backupPath)

	if err := os.Rename(self, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(newBin, self); err != nil {
		_ = os.Rename(backupPath, self)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	if err := verifyBinaryRuns(self); err != nil {
		_ = os.Remove(self)
		if restoreErr := os.Rename(backupPath, self); restoreErr != nil {
			return fmt.Errorf("post-install verification failed and could not restore backup: %v (verify error: %w)", restoreErr, err)
		}
		return fmt.Errorf("post-install verification failed; restored previous binary: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}
