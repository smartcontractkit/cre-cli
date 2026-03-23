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
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

var (
	ErrNoCredentials    = errors.New("no credentials found")
	ErrValidationFailed = errors.New("credential validation failed")
)

type Context struct {
	Logger         *zerolog.Logger
	Viper          *viper.Viper
	ClientFactory  client.Factory
	Settings       *settings.Settings
	Credentials    *credentials.Credentials
	EnvironmentSet *environments.EnvironmentSet
	TenantContext  *tenantctx.EnvironmentContext
	Workflow       WorkflowRuntime
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
		if err := validator.ValidateCredentials(validationCtx, ctx.Credentials); err != nil {
			return fmt.Errorf("%w: %w", ErrValidationFailed, err)
		}
	}

	return nil
}

// AttachTenantContext ensures context.yaml exists (fetching if needed) and loads
// the tenant context for the current environment into the runtime context.
// This does not modify EnvironmentSet — that will happen in a future phase.
func (ctx *Context) AttachTenantContext(validationCtx context.Context) error {
	if ctx.Credentials == nil || ctx.EnvironmentSet == nil {
		return fmt.Errorf("credentials and environment must be loaded before tenant context")
	}

	if err := tenantctx.EnsureContext(validationCtx, ctx.Credentials, ctx.EnvironmentSet, ctx.Logger); err != nil {
		return fmt.Errorf("failed to ensure tenant context: %w", err)
	}

	envName := ctx.EnvironmentSet.EnvName
	if envName == "" {
		envName = environments.DefaultEnv
	}

	envCtx, err := tenantctx.LoadContext(envName)
	if err != nil {
		return fmt.Errorf("failed to load tenant context: %w", err)
	}

	ctx.TenantContext = envCtx
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
