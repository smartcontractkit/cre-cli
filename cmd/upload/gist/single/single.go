package single

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/cmd/gist"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	GithubToken gist.GitHubAPIToken `validate:"required"`
	FilePath    string              `validate:"required,filepath"`
	GistID      string              `validate:"omitempty,gist_id"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var singleUploadCmd = &cobra.Command{
		Use:    "single fileName.ext",
		Short:  "Uploads single file to a Github Gist",
		Hidden: true, // Hide this command from the help output, unhide after M2 release
		Long:   `Uploads one file to a new Gist or updates existing Gist`,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)
			inputs, err := handler.ResolveInputs(runtimeContext.Viper, args)
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

	singleUploadCmd.Flags().StringP("gist-id", "g", "", "Use to specify the Gist ID to update")

	return singleUploadCmd
}

type handler struct {
	log           *zerolog.Logger
	clientFactory client.Factory
	settings      *settings.Settings
	validated     bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:           ctx.Logger,
		clientFactory: ctx.ClientFactory,
		settings:      ctx.Settings,
		validated:     false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper, args []string) (Inputs, error) {
	return Inputs{
		GithubToken: h.settings.StorageSettings.Gist.GithubToken,
		FilePath:    args[0],
		GistID:      v.GetString("gist-id"),
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err = validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	if inputs.GithubToken.RawValue() == "" {
		return errors.New("Github API token missing")
	}

	filePath, err := filepath.Abs(inputs.FilePath)
	if err != nil {
		return fmt.Errorf("absolute file path error: %w", err)
	}

	inputs.FilePath = filePath
	h.validated = true
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	var gistUrl string
	var err error
	if inputs.GistID == "" {
		gistUrl, err = h.createNewGist(inputs.FilePath, inputs.GithubToken)
		if err != nil {
			return fmt.Errorf("Gist not created: %w", err)
		}
	} else {
		gistUrl, err = h.updateExistingGist(inputs.FilePath, inputs.GistID, inputs.GithubToken)
		if err != nil {
			return fmt.Errorf("Gist not updated: %w", err)
		}
	}

	h.log.Info().Str("Gist URL", gistUrl).Str("File name", filepath.Base(inputs.FilePath)).Msg("Successfully uploaded file to Gist")
	return nil
}

func (h *handler) createNewGist(filePath string, token gist.GitHubAPIToken) (string, error) {
	h.log.Info().Str("File", filePath).Msg("Verifying if file contains binary content...")
	isBinary, err := cmdCommon.IsBinaryFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Gist not created: %w", err)
	}

	h.log.Info().Bool("Binary content", isBinary).Msg("Creating new Gist...")
	if isBinary {
		return gist.CreateGistForBinaryContent(h.log, "Created by CRE CLI", filePath, false, token)
	}

	return gist.CreateGistForTextualContent(h.log, "Created by CRE CLI", filePath, false, token)
}

func (h *handler) updateExistingGist(filePath string, gistID string, token gist.GitHubAPIToken) (string, error) {
	h.log.Info().Str("Gist ID", gistID).Msg("Verifying if Gist contains binary content...")
	isBinary, err := gist.IsGistBinaryContent(h.log, gistID, token)
	if err != nil {
		return "", fmt.Errorf("Gist not updated: %w", err)
	}

	h.log.Info().Str("Gist ID", gistID).Bool("Binary content", isBinary).Msg("Updating existing Gist...")
	if isBinary {
		return gist.UpdateGistForBinaryContent(h.log, gistID, "Updated by Chainlink CLI", filePath, false, token)
	}

	return gist.UpdateGistForTextualContent(h.log, gistID, "Updated by Chainlink CLI", filePath, false, token)
}
