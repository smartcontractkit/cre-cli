package logout

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/credentials"
	"github.com/smartcontractkit/dev-platform/internal/environments"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
)

var (
	revokeURL = ""
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Revoke authentication tokens and remove local credentials",
		Long:  "Invalidates the current authentication tokens and deletes stored credentials.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx)
			return h.execute()
		},
	}
	return cmd
}

type handler struct {
	log            *zerolog.Logger
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
	}
}

func (h *handler) execute() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	credPath := filepath.Join(home, credentials.ConfigDir, credentials.ConfigFile)

	if h.credentials.Tokens == nil {
		h.log.Info().Msg("user not logged in")
		return nil
	}

	if h.credentials.AuthType == credentials.AuthTypeBearer && h.credentials.Tokens.RefreshToken != "" {
		h.log.Debug().Msg("Revoking refresh token")
		form := url.Values{}
		form.Set("token", h.credentials.Tokens.RefreshToken)
		form.Set("client_id", h.environmentSet.ClientID)

		if revokeURL == "" {
			revokeURL = h.environmentSet.AuthBase + constants.AuthRevokePath
		}
		// #nosec G107: revokeURL is a validated constant from configuration
		resp, err := http.PostForm(revokeURL, form)
		if err != nil {
			h.log.Warn().Err(err).Msg("Failed to revoke refresh token")
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				h.log.Warn().Msgf("Revocation failed with status %d", resp.StatusCode)
			} else {
				h.log.Debug().Msg("Refresh token revoked")
			}
		}
	}

	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials file: %w", err)
	}

	h.log.Info().Msg("Logged out successfully")
	return nil
}
