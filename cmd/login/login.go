package login

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	rt "runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/oauth"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

var (
	// OrgMembershipErrorSubstring is the error message substring returned by Auth0
	// when a user doesn't belong to any organization during the auth flow.
	// This typically happens during sign-up when the organization hasn't been created yet.
	OrgMembershipErrorSubstring = "user does not belong to any organization"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Start authentication flow",
		Long: `Opens a browser for interactive login and saves credentials.

For non-interactive environments (CI/CD, automation, AI agents), set the
CRE_API_KEY environment variable instead:

  export CRE_API_KEY=<your-api-key>

API keys can be created at https://app.chain.link (see Account Settings).
When CRE_API_KEY is set, all commands that require authentication will use
it automatically — no login needed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.New()
			if err := v.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if v.GetBool(settings.Flags.NonInteractive.Name) {
				ui.ErrorWithSuggestions(
					"Login requires a browser and is not available in non-interactive mode",
					[]string{
						"Set CRE_API_KEY environment variable instead: export CRE_API_KEY=<your-api-key>",
						"API keys can be created at https://app.chain.link (Account Settings)",
					},
				)
				return fmt.Errorf("login is not supported in non-interactive mode, use CRE_API_KEY instead")
			}
			h := newHandler(runtimeCtx)
			return h.execute()
		},
	}

	cmd.Flags().Bool(settings.Flags.NonInteractive.Name, false, "Fail instead of prompting; requires all inputs via flags")

	return cmd
}

// Run executes the login flow directly without going through Cobra.
// This is useful for prompting login from other commands when auth is required.
func Run(runtimeCtx *runtime.Context) error {
	h := newHandler(runtimeCtx)
	return h.execute()
}

type handler struct {
	environmentSet   *environments.EnvironmentSet
	log              *zerolog.Logger
	lastPKCEVerifier string
	lastState        string
	retryCount       int
	spinner          *ui.Spinner
}

const maxOrgNotFoundRetries = 3

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		environmentSet: ctx.EnvironmentSet,
		spinner:        ui.NewSpinner(),
	}
}

func (h *handler) execute() error {
	// Welcome message (no spinner yet)
	ui.Title("CRE Login")
	ui.Line()
	ui.Dim("Authenticate with your Chainlink account")
	ui.Line()

	code, err := h.startAuthFlow()
	if err != nil {
		h.spinner.StopAll()
		return err
	}

	// Use spinner for the token exchange
	h.spinner.Start("Exchanging authorization code...")
	tokenSet, err := oauth.ExchangeAuthorizationCode(context.Background(), nil, h.environmentSet, code, h.lastPKCEVerifier, "", "")
	if err != nil {
		h.spinner.StopAll()
		h.log.Error().Err(err).Msg("code exchange failed")
		return err
	}

	h.spinner.Update("Saving credentials...")
	if err := credentials.SaveCredentials(tokenSet); err != nil {
		h.spinner.StopAll()
		h.log.Error().Err(err).Msg("failed to save credentials")
		return err
	}

	h.spinner.Update("Fetching user context...")
	if err := h.fetchTenantConfig(tokenSet); err != nil {
		h.log.Debug().Err(err).Msgf("failed to fetch user context — %s not written", tenantctx.ContextFile)
	}

	// Stop spinner before final output
	h.spinner.Stop()

	ui.Line()
	ui.Success("Login completed successfully!")
	ui.EnvContext(h.environmentSet.EnvLabel())
	ui.Line()

	// Show next steps in a styled box
	nextSteps := ui.RenderBold("Next steps:") + "\n" +
		"  " + ui.RenderCommand("cre init") + "  Create a new CRE project\n" +
		"  " + ui.RenderCommand("cre whoami") + "  View your account info"
	ui.Box(nextSteps)
	ui.Line()

	return nil
}

func (h *handler) startAuthFlow() (string, error) {
	codeCh := make(chan string, 1)

	// Use spinner while setting up server
	h.spinner.Start("Preparing authentication...")

	server, listener, err := h.setupServer(codeCh)
	if err != nil {
		h.spinner.Stop()
		return "", err
	}
	defer func() {
		if err := server.Shutdown(context.Background()); err != nil {
			h.log.Warn().Err(err).Msg("error shutting down login callback server")
		}
	}()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			h.log.Error().Err(err).Msg("server error")
		}
	}()

	verifier, challenge, err := oauth.GeneratePKCE()
	if err != nil {
		h.spinner.Stop()
		return "", err
	}
	h.lastPKCEVerifier = verifier
	state, err := oauth.RandomState()
	if err != nil {
		h.spinner.Stop()
		return "", err
	}
	h.lastState = state

	authURL := h.buildAuthURL(challenge, h.lastState)

	// Stop spinner before showing URL (static content)
	h.spinner.Stop()

	// Show URL - this stays visible while user authenticates in browser
	ui.Step("Opening browser to:")
	ui.URL(authURL)
	ui.Line()

	if err := oauth.OpenBrowser(authURL, rt.GOOS); err != nil {
		ui.Warning("Could not open browser automatically")
		ui.Dim("Please open the URL above in your browser")
		ui.Line()
	}

	// Static waiting message (no spinner - user will see this when they return)
	ui.Dim("Waiting for authentication... (Press Ctrl+C to cancel)")

	select {
	case code := <-codeCh:
		ui.Line()
		return code, nil
	case <-time.After(500 * time.Second):
		return "", fmt.Errorf("timeout waiting for authorization code")
	}
}

func (h *handler) setupServer(codeCh chan string) (*http.Server, net.Listener, error) {
	return oauth.NewCallbackHTTPServer(constants.AuthListenAddr, h.callbackHandler(codeCh))
}

func (h *handler) callbackHandler(codeCh chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for error in the callback (Auth0 error responses)
		errorParam := r.URL.Query().Get("error")
		errorDesc := r.URL.Query().Get("error_description")

		if errorParam != "" {
			// Check if this is an organization membership error
			if strings.Contains(errorDesc, OrgMembershipErrorSubstring) {
				if h.retryCount >= maxOrgNotFoundRetries {
					h.log.Error().Int("retries", h.retryCount).Msg("organization setup timed out after maximum retries")
					oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusBadRequest)
					return
				}

				// Generate new authentication credentials for the retry
				verifier, challenge, err := oauth.GeneratePKCE()
				if err != nil {
					h.log.Error().Err(err).Msg("failed to prepare authentication retry")
					oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusInternalServerError)
					return
				}
				h.lastPKCEVerifier = verifier
				st, err := oauth.RandomState()
				if err != nil {
					h.log.Error().Err(err).Msg("failed to generate OAuth state for retry")
					oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusInternalServerError)
					return
				}
				h.lastState = st
				h.retryCount++

				// Build the new auth URL for redirect
				authURL := h.buildAuthURL(challenge, h.lastState)

				h.log.Debug().Int("attempt", h.retryCount).Int("max", maxOrgNotFoundRetries).Msg("organization setup in progress, retrying")
				oauth.ServeWaitingPage(h.log, w, authURL)
				return
			}

			// Generic Auth0 error
			h.log.Error().Str("error", errorParam).Str("description", errorDesc).Msg("auth error in callback")
			oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusBadRequest)
			return
		}

		if st := r.URL.Query().Get("state"); st == "" || h.lastState == "" || st != h.lastState {
			h.log.Error().Msg("invalid state in response")
			oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			h.log.Error().Msg("no code in response")
			oauth.ServeEmbeddedHTML(h.log, w, oauth.PageError, http.StatusBadRequest)
			return
		}

		oauth.ServeEmbeddedHTML(h.log, w, oauth.PageSuccess, http.StatusOK)
		codeCh <- code
	}
}

func (h *handler) buildAuthURL(codeChallenge, state string) string {
	params := url.Values{}
	params.Set("client_id", h.environmentSet.ClientID)
	params.Set("redirect_uri", constants.AuthRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid profile email offline_access")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	if h.environmentSet.Audience != "" {
		params.Set("audience", h.environmentSet.Audience)
	}
	params.Set("state", state)

	return h.environmentSet.AuthBase + constants.AuthAuthorizePath + "?" + params.Encode()
}

func (h *handler) fetchTenantConfig(tokenSet *credentials.CreLoginTokenSet) error {
	creds := &credentials.Credentials{
		Tokens:   tokenSet,
		AuthType: credentials.AuthTypeBearer,
	}
	gqlClient := graphqlclient.New(creds, h.environmentSet, h.log)

	envName := h.environmentSet.EnvName
	if envName == "" {
		envName = environments.DefaultEnv
	}

	return tenantctx.FetchAndWriteContext(context.Background(), gqlClient, envName, h.log)
}
