package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/dev-platform/internal/runtime"
	"github.com/smartcontractkit/dev-platform/internal/validation"
)

type Inputs struct {
	TestDirectory string `validate:"required,dir,path_read"`
	TestName      string
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var testCmd = &cobra.Command{
		Use:    "test ./test-directory",
		Short:  "Runs Go tests under the specified workflow package/directory",
		Hidden: true, // Hide this command from the help output, unhide after M2 release
		Long:   `This command runs "go test" on the specified Go package to execute tests.`,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(args, runtimeContext.Viper)
			if err != nil {
				return err
			}
			err = handler.ValidateInputs(inputs)
			if err != nil {
				return err
			}
			return handler.Execute(inputs)
		},
	}

	testCmd.Flags().StringP("run", "r", "", "Runs only tests and examples matching provided regular expression")

	return testCmd
}

type handler struct {
	log       *zerolog.Logger
	validated bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:       ctx.Logger,
		validated: false,
	}
}

func (h *handler) ResolveInputs(args []string, v *viper.Viper) (Inputs, error) {
	return Inputs{
		TestDirectory: args[0],
		TestName:      v.GetString("run"),
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err := validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	h.validated = true
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	absDir, err := filepath.Abs(inputs.TestDirectory)
	h.log.Info().Str("Directory path", absDir).Msg("Loading the test directory")
	if err != nil {
		return fmt.Errorf("failed to get absolute path for the test directory: %w", err)
	}

	// Prepare the "go test" command
	testArgs := []string{"test", "-v", absDir}

	// If a test name is provided, add it to the test command, otherwise run all tests.
	testName := inputs.TestName
	if testName != "" {
		testArgs = append(testArgs, "-run", testName)
	}

	// Run the "go test" command on the specified directory
	testCmd := exec.Command("go", testArgs...)
	testCmd.Env = append(os.Environ(), "LOG_LEVEL="+h.log.GetLevel().String())
	testCmd.Stdout = os.Stdout // Capture stdout as is
	testCmd.Stderr = os.Stderr // Capture stderr as is

	h.log.Info().Msg("Running the tests...")

	// Run the test command
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("failed to run tests: %w", err)
	}

	h.log.Info().Msg("Test execution complete")
	return nil
}
