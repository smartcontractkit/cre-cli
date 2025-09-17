package batch

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
	FilePaths   []string            `validate:"required,dive,filepath"`
	GistIDs     []string            `validate:"omitempty,dive,gist_id"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var batchUploadCmd = &cobra.Command{
		Use:    "batch",
		Short:  "Uploads multiple files to multiple Github Gists",
		Hidden: true, // Hide this command from the help output, unhide after M2 release
		Long:   `Uploads one or more files to existing Github Gists (content update) or create new Gists for multiple files`,
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

	batchUploadCmd.Flags().StringArrayP("file", "f", []string{}, "Path to files for upload [REQUIRED]")
	if err := batchUploadCmd.MarkFlagRequired("file"); err != nil {
		runtimeContext.Logger.Fatal().Err(err).Msg("Failed to set 'files' flag as required")
	}
	batchUploadCmd.Flags().StringArrayP("gist-id", "g", []string{}, "One or more existing Gist IDs to update (order should match the order of files)")

	return batchUploadCmd
}

type handler struct {
	log           *zerolog.Logger
	clientFactory client.Factory
	v             *viper.Viper
	settings      *settings.Settings
	validated     bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:           ctx.Logger,
		clientFactory: ctx.ClientFactory,
		v:             ctx.Viper,
		settings:      ctx.Settings,
		validated:     false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper, args []string) (Inputs, error) {
	filePaths := v.GetStringSlice("file")
	gistIDs := v.GetStringSlice("gist-id")

	return Inputs{
		GithubToken: h.settings.StorageSettings.Gist.GithubToken,
		FilePaths:   filePaths,
		GistIDs:     gistIDs,
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

	h.log.Info().Msg("Validating all provided file paths")
	for i, file := range inputs.FilePaths {
		absPath, err := filepath.Abs(file)
		h.log.Info().Str("File path", file).Msg("Searching for the file to upload")
		if err != nil {
			return fmt.Errorf("absolute file path error: %w", err)
		}

		// Update the file path to absolute path
		inputs.FilePaths[i] = absPath
	}

	if len(inputs.GistIDs) > 0 {
		h.log.Info().Msg("Detected that Gist IDs are provided, validating Gists")
		if len(inputs.GistIDs) != len(inputs.FilePaths) {
			return errors.New("Files and Gist IDs mismatch")
		}

		for _, gistID := range inputs.GistIDs {
			h.log.Info().Str("Gist ID", gistID).Msg("Validating Gist ID...")
			if !gist.IsValidGistID(gistID) {
				return errors.New("Invalid Gist ID")
			}
		}
	}

	h.validated = true
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	for index, filePath := range inputs.FilePaths {
		var gistUrl string
		var err error

		if len(inputs.GistIDs) == 0 {
			gistUrl, err = h.createNewGist(filePath, inputs.GithubToken)
			if err != nil {
				return fmt.Errorf("Gist not created: %w", err)
			}
		} else {
			gistUrl, err = h.updateExistingGist(filePath, inputs.GistIDs[index], inputs.GithubToken)
			if err != nil {
				return fmt.Errorf("Gist not updated: %w", err)
			}
		}

		h.log.Info().Str("Gist URL", gistUrl).Str("File name", filepath.Base(filePath)).Msg("Successfully uploaded file to Gist")
	}

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
