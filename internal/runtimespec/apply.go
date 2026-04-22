package runtimespec

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	intcontext "github.com/smartcontractkit/cre-cli/internal/context"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// Apply runs the attach steps selected in spec in a fixed dependency order.
// The root command calls this from PersistentPreRunE for all paths; a nil or
// empty spec is a no-op.
func Apply(ctx context.Context, rt *runtime.Context, cmd *cobra.Command, args []string, spec *runtime.AttachConfig) error {
	if spec == nil || spec.IsEmpty() {
		return nil
	}

	if spec.Environment && rt.EnvironmentSet == nil {
		if err := rt.AttachEnvironmentSet(); err != nil {
			return fmt.Errorf("load environment: %w", err)
		}
	}

	if spec.Credentials {
		if spec.SkipCredentialValidation {
			if err := rt.AttachCredentials(cmd.Context(), true); err != nil {
				return err
			}
		} else {
			if err := attachCredentialsInteractive(cmd, rt); err != nil {
				return err
			}
		}
	}

	if spec.TenantContext {
		if err := rt.AttachTenantContext(ctx); err != nil {
			rt.Logger.Warn().Err(err).Msg("failed to load user context — context.yaml not available")
		}
	}

	if spec.ExecutionContext {
		if rt.InvocationDir == "" {
			if invocationDir, err := os.Getwd(); err == nil {
				rt.InvocationDir = invocationDir
			}
		}
		projectRootFlag := rt.Viper.GetString(settings.Flags.ProjectRoot.Name)
		if err := intcontext.SetExecutionContext(cmd, args, projectRootFlag, rt.Logger); err != nil {
			return err
		}
	}

	if spec.NeedsSettingsLoad() {
		// Defer ValidateDeploymentRPC inside load until after we know registry type;
		// pass false so private/off-chain registries are not forced to define chain RPCs.
		if err := rt.AttachSettings(cmd, false); err != nil {
			return fmt.Errorf("load settings: %w", err)
		}
	}

	if spec.ResolveWorkflowOwner && !spec.ResolvedRegistry {
		return fmt.Errorf("internal: ResolveWorkflowOwner requires ResolvedRegistry in attach spec")
	}

	if spec.ResolvedRegistry {
		if err := rt.AttachResolvedRegistry(cmd, spec.ResolveWorkflowOwner); err != nil {
			return err
		}
	}

	if spec.ValidateDeploymentRPC && rt.ResolvedRegistry != nil && rt.Settings != nil && rt.EnvironmentSet != nil {
		if rt.ResolvedRegistry.Type() == settings.RegistryTypeOnChain {
			if err := settings.ValidateDeploymentRPC(&rt.Settings.Workflow, rt.EnvironmentSet.WorkflowRegistryChainName); err != nil {
				return err
			}
		}
	}

	return nil
}
