package runtimespec

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/login"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// attachCredentialsInteractive loads credentials and, when needed, runs the
// same login prompt flow as root PersistentPreRunE.
func attachCredentialsInteractive(cmd *cobra.Command, rt *runtime.Context) error {
	skipValidation := false // matches shouldSkipValidation for non-logout commands in root
	err := rt.AttachCredentials(cmd.Context(), skipValidation)
	if err != nil {
		if errors.Is(err, runtime.ErrValidationFailed) {
			ui.Line()
			if rt.EnvironmentSet != nil && rt.EnvironmentSet.RequiresVPN() {
				ui.ErrorWithSuggestions("Credential validation failed", []string{
					fmt.Sprintf("The %s environment requires Tailscale VPN.", rt.EnvironmentSet.EnvName),
					"Ensure Tailscale is connected to the smartcontract.com network, then retry.",
				})
			} else {
				ui.Error("Credential validation failed")
			}
			if rt.EnvironmentSet != nil {
				ui.EnvContext(rt.EnvironmentSet.EnvLabel())
			}
			ui.Line()
			return fmt.Errorf("authentication required: %w", err)
		}

		if errors.Is(err, runtime.ErrNoCredentials) {
			ui.Line()
			ui.Warning("You are not logged in")
			if rt.EnvironmentSet != nil {
				ui.EnvContext(rt.EnvironmentSet.EnvLabel())
			}
			ui.Line()

			runLogin, formErr := ui.Confirm("Would you like to login now?",
				ui.WithLabels("Yes, login", "No, cancel"),
			)
			if formErr != nil {
				return fmt.Errorf("authentication required: %w", err)
			}

			if !runLogin {
				return fmt.Errorf("authentication required: %w", err)
			}

			ui.Line()
			if loginErr := login.Run(rt); loginErr != nil {
				return fmt.Errorf("login failed: %w", loginErr)
			}
			return runtime.ErrLoginCompleted
		}

		return err
	}

	return nil
}
