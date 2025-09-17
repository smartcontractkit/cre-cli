package client

import (
	"fmt"
	"strings"
)

func DecodeCustomError(errStr string) string {
	// example:
	// error type: WorkflowDoesNotExist, error values: []: execution reverted
	if !strings.Contains(errStr, "execution reverted") {
		return fmt.Sprintf("Transaction revert not detected, error found: %s", errStr)
	}

	start := strings.Index(errStr, "error type: ")
	if start == -1 {
		return fmt.Sprintf("Revert type not detected, error found: %s", errStr)
	}
	start += len("error type: ")

	end := strings.Index(errStr[start:], ",")
	if end == -1 {
		return fmt.Sprintf("Revert type not detected, error found: %s", errStr)
	}

	return errStr[start : start+end]
}
