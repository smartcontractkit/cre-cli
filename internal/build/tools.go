package build

import (
	"errors"
	"fmt"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

func EnsureToolsForBuild(workflowLanguage string) error {
	switch workflowLanguage {
	case constants.WorkflowLanguageTypeScript:
		if err := cmdcommon.EnsureTool("bun"); err != nil {
			return errors.New("bun is required for TypeScript workflows but was not found in PATH; install from https://bun.com/docs/installation")
		}
	case constants.WorkflowLanguageGolang:
		if err := cmdcommon.EnsureTool("go"); err != nil {
			return errors.New("go toolchain is required for Go workflows but was not found in PATH; install from https://go.dev/dl")
		}
	default:
		return fmt.Errorf("unsupported workflow workflowLanguage %s", workflowLanguage)
	}
	return nil
}
