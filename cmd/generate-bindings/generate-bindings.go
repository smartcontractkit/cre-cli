package generatebindings

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/spf13/cobra"
)

// runCommand executes a command in a specified directory
func runCommand(dir string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", command, err)
	}

	return nil
}

type EvmInputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	ChainFamily string `validate:"required,oneof=evm" cli:"--chain-family"`
	Language    string `validate:"required,oneof=go" cli:"--language"`
	// just keeping it simple for now
	AbiPath string `validate:"required,path_read" cli:"--abi"`
	PkgName string `validate:"required" cli:"--pkg"`
	OutPath string `validate:"required" cli:"--out"`
}

func NewEvmBindings(runtimeContext *runtime.Context) *cobra.Command {
	generateBindingsCmd := &cobra.Command{
		Use:   "generate-bindings <chain-family>",
		Short: "Generate bindings from contract ABI",
		Long: `This command generates bindings from contract ABI files.
Supports EVM chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, IERC20.abi generates bindings in generated/ierc20/ package.`,
		Example: "  cre generate-bindings evm",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveEvmInputs(args, runtimeContext.Viper)
			if err != nil {
				return err
			}
			if err := validateEvmInputs(inputs); err != nil {
				return err
			}
			return executeEvm(inputs)
		},
	}

	generateBindingsCmd.Flags().StringP("project-root", "p", "", "Path to project root directory (defaults to current directory)")
	generateBindingsCmd.Flags().StringP("language", "l", "go", "Target language (go)")
	generateBindingsCmd.Flags().StringP("abi", "a", "", "Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)")
	generateBindingsCmd.Flags().StringP("pkg", "k", "bindings", "Base package name (each contract gets its own subdirectory)")

	return generateBindingsCmd
}

type SolanaInputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	Language    string `validate:"required,oneof=go" cli:"--language"`
	// just keeping it simple for now
	IdlPath string `validate:"required,path_read" cli:"--idl"`
	// PkgName string `validate:"required" cli:"--pkg"`
	OutPath string `validate:"required" cli:"--out"`
}

func NewSolanaBindings(runtimeContext *runtime.Context) *cobra.Command {
	var generateBindingsCmd = &cobra.Command{
		Use:   "generate-bindings-solana",
		Short: "Generate bindings from contract IDL",
		Long: `This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.`,
		Example: "  cre generate-bindings-solana",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveSolanaInputs(args, runtimeContext.Viper)
			if err != nil {
				return err
			}
			if err := validateSolanaInputs(inputs); err != nil {
				return err
			}
			return executeSolana(inputs)
		},
	}

	generateBindingsCmd.Flags().StringP("project-root", "p", "", "Path to project root directory (defaults to current directory)")
	generateBindingsCmd.Flags().StringP("language", "l", "go", "Target language (go)")
	generateBindingsCmd.Flags().StringP("abi", "a", "", "Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)")
	generateBindingsCmd.Flags().StringP("pkg", "k", "bindings", "Base package name (each contract gets its own subdirectory)")

	return generateBindingsCmd
}
