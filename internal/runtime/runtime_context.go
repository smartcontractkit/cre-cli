package runtime

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
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
	ProjectRootDir string
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

func (ctx *Context) AttachCredentials() error {
	var err error

	ctx.Credentials, err = credentials.New(ctx.Logger)
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
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
