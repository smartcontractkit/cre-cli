package runtime

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/authvalidation"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

type Context struct {
	Logger         *zerolog.Logger
	Viper          *viper.Viper
	ClientFactory  client.Factory
	Settings       *settings.Settings
	Credentials    *credentials.Credentials
	EnvironmentSet *environments.EnvironmentSet
	Workflow       WorkflowRuntime
}

type WorkflowRuntime struct {
	ID string
}

func NewContext(logger *zerolog.Logger, viper *viper.Viper) *Context {
	factory := client.NewFactory(logger, viper)

	return &Context{
		Logger:        logger,
		Viper:         viper,
		ClientFactory: factory,
	}
}

func (ctx *Context) AttachSettings(cmd *cobra.Command) error {
	var err error

	ctx.Settings, err = settings.New(ctx.Logger, ctx.Viper, cmd)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	return nil
}

func (ctx *Context) AttachCredentials(validationCtx context.Context, skipValidation bool) error {
	var err error

	ctx.Credentials, err = credentials.New(ctx.Logger)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Validate credentials immediately after loading (unless skipped)
	if !skipValidation {
		if ctx.EnvironmentSet == nil {
			return fmt.Errorf("failed to load environment")
		}

		validator := authvalidation.NewValidator(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
		if err := validator.ValidateCredentials(validationCtx, ctx.Credentials); err != nil {
			return fmt.Errorf("authentication validation failed: %w", err)
		}
	}

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
