package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/authvalidation"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

var (
	ErrNoCredentials    = errors.New("no credentials found")
	ErrValidationFailed = errors.New("credential validation failed")
)

type Context struct {
	Logger           *zerolog.Logger
	Viper            *viper.Viper
	ClientFactory    client.Factory
	Settings         *settings.Settings
	Credentials      *credentials.Credentials
	EnvironmentSet   *environments.EnvironmentSet
	TenantContext    *tenantctx.EnvironmentContext
	ResolvedRegistry settings.ResolvedRegistry
	Workflow         WorkflowRuntime

	OrgID                string
	DerivedWorkflowOwner string
	// InvocationDir is the working directory at the time the CLI was invoked,
	// before any os.Chdir calls made by SetExecutionContext.
	InvocationDir string
}

type WorkflowRuntime struct {
	ID       string
	Language string
}

func NewContext(logger *zerolog.Logger, viper *viper.Viper) *Context {
	factory := client.NewFactory(logger, viper)

	return &Context{
		Logger:        logger,
		Viper:         viper,
		ClientFactory: factory,
	}
}

func (ctx *Context) AttachSettings(cmd *cobra.Command, validateDeployRPC bool) error {
	var err error
	registryChainName := ""

	if validateDeployRPC {
		registryChainName = ctx.EnvironmentSet.WorkflowRegistryChainName
	}

	ctx.Settings, err = settings.New(ctx.Logger, ctx.Viper, cmd, registryChainName)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	return nil
}

// FinalizeDeferredWorkflowOwner fills workflow owner when settings load deferred it
// (non-empty deployment-registry). Call after AttachResolvedRegistry.
func (ctx *Context) FinalizeDeferredWorkflowOwner(cmd *cobra.Command) error {
	if ctx.Settings == nil {
		return nil
	}
	return settings.FinalizeWorkflowOwner(
		ctx.Viper,
		cmd,
		&ctx.Settings.Workflow,
		ctx.Settings.User.TargetName,
		ctx.ResolvedRegistry,
		ctx.DerivedWorkflowOwner,
	)
}

func (ctx *Context) AttachCredentials(validationCtx context.Context, skipValidation bool) error {
	var err error

	ctx.Credentials, err = credentials.New(ctx.Logger)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoCredentials, err)
	}

	if !skipValidation {
		if ctx.EnvironmentSet == nil {
			return fmt.Errorf("%w: failed to load environment", ErrValidationFailed)
		}

		validator := authvalidation.NewValidator(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
		result, err := validator.ValidateCredentials(validationCtx, ctx.Credentials)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrValidationFailed, err)
		}

		if result != nil {
			ctx.OrgID = result.OrgID
			ctx.DerivedWorkflowOwner = ethkeys.FormatWorkflowOwnerAddress(result.DerivedWorkflowOwner)
		}
	}

	return nil
}

// AttachTenantContext loads the user context for the current environment.
// If the manifest is missing, it is fetched from the service first.
func (ctx *Context) AttachTenantContext(validationCtx context.Context) error {
	if ctx.Credentials == nil || ctx.EnvironmentSet == nil {
		return fmt.Errorf("credentials and environment must be loaded before user context")
	}

	if err := tenantctx.EnsureContext(validationCtx, ctx.Credentials, ctx.EnvironmentSet, ctx.Logger); err != nil {
		return fmt.Errorf("failed to ensure user context: %w", err)
	}

	envName := ctx.EnvironmentSet.EnvName
	if envName == "" {
		envName = environments.DefaultEnv
	}

	envCtx, err := tenantctx.LoadContext(envName)
	if err != nil {
		return fmt.Errorf("failed to load user context: %w", err)
	}

	ctx.TenantContext = envCtx
	return nil
}

// AttachResolvedRegistry resolves the deployment-registry from workflow
// settings against the tenant context registries. Must be called after
// AttachSettings and AttachTenantContext.
func (ctx *Context) AttachResolvedRegistry() error {
	deploymentRegistry := ""
	if ctx.Settings != nil {
		deploymentRegistry = ctx.Settings.Workflow.UserWorkflowSettings.DeploymentRegistry
	}

	resolved, err := settings.ResolveRegistry(deploymentRegistry, ctx.TenantContext, ctx.EnvironmentSet)
	if err != nil {
		return fmt.Errorf("failed to resolve deployment registry: %w", err)
	}

	ctx.ResolvedRegistry = resolved
	return nil
}

func (ctx *Context) AttachEnvironmentSet() error {
	var err error

	ctx.EnvironmentSet, err = environments.New()
	if err != nil {
		return fmt.Errorf("failed to load environment details: %w", err)
	}

	return nil
}
