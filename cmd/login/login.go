package login

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	rt "runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

var (
	httpClient  = &http.Client{Timeout: 10 * time.Second}
	tokenURL    = ""
	errorPage   = "htmlPages/error.html"
	successPage = "htmlPages/success.html"
	stylePage   = "htmlPages/output.css"
)

//go:embed htmlPages/*.html
//go:embed htmlPages/*.css
var htmlFiles embed.FS

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Start authentication flow",
		Long:  "Opens browser for user login and saves credentials.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx)
			return h.execute()
		},
	}

	return cmd
}

type handler struct {
	environmentSet *environments.EnvironmentSet
	log            *zerolog.Logger
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		environmentSet: ctx.EnvironmentSet,
	}
}

func (h *handler) execute() error {
	code, err := h.startAuthFlow()
	if err != nil {
		return err
	}

	tokenSet, err := h.exchangeCodeForTokens(context.Background(), code)
	if err != nil {
		h.log.Error().Err(err).Msg("code exchange failed")
		return err
	}

	if err := saveCredentials(tokenSet); err != nil {
		h.log.Error().Err(err).Msg("failed to save credentials")
		return err
	}

	h.log.Info().Msg("Login completed successfully")
	return nil
}

func (h *handler) startAuthFlow() (string, error) {
	codeCh := make(chan string, 1)

	server, listener, err := h.setupServer(codeCh)
	if err != nil {
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

	authURL := h.buildAuthURL()
	h.log.Info().Msgf("Opening browser to %s", authURL)
	if err := openBrowser(authURL); err != nil {
		h.log.Warn().Err(err).Msg("could not open browser, please navigate manually")
	}

	select {
	case code := <-codeCh:
		return code, nil
	case <-time.After(500 * time.Second):
		return "", fmt.Errorf("timeout waiting for authorization code")
	}
}

func (h *handler) setupServer(codeCh chan string) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", h.callbackHandler(codeCh))

	// TODO: Add a fallback port in case the default port is in use
	listener, err := net.Listen("tcp", constants.CognitoAuthListenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %w", constants.CognitoAuthListenAddr, err)
	}

	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}, listener, nil
}

func (h *handler) callbackHandler(codeCh chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			h.log.Error().Msg("no code in response")
			h.serveEmbeddedHTML(w, errorPage, http.StatusBadRequest)
			return
		}

		h.serveEmbeddedHTML(w, successPage, http.StatusOK)
		codeCh <- code
	}
}

func (h *handler) serveEmbeddedHTML(w http.ResponseWriter, filePath string, status int) {
	htmlContent, err := htmlFiles.ReadFile(filePath)
	if err != nil {
		h.log.Error().Err(err).Str("file", filePath).Msg("failed to read embedded HTML file")
		h.sendHTTPError(w)
		return
	}

	cssContent, err := htmlFiles.ReadFile(stylePage)
	if err != nil {
		h.log.Error().Err(err).Str("file", stylePage).Msg("failed to read embedded CSS file")
		h.sendHTTPError(w)
		return
	}

	modified := strings.Replace(
		string(htmlContent),
		`<link rel="stylesheet" href="./output.css" />`,
		fmt.Sprintf("<style>%s</style>", string(cssContent)),
		1,
	)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(modified)); err != nil {
		h.log.Error().Err(err).Msg("failed to write HTML response")
	}
}

func (h *handler) sendHTTPError(w http.ResponseWriter) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func (h *handler) buildAuthURL() string {
	params := url.Values{}
	params.Set("client_id", h.environmentSet.ClientID)
	params.Set("redirect_uri", constants.CognitoAuthRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid profile email aws.cognito.signin.user.admin")
	if tokenURL == "" {
		tokenURL = h.environmentSet.UIURL + constants.CreUiAuthPath
	}

	return tokenURL + "?" + params.Encode()
}

func (h *handler) exchangeCodeForTokens(ctx context.Context, code string) (*credentials.CreLoginTokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", h.environmentSet.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", constants.CognitoAuthRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.environmentSet.CognitoURL+constants.CognitoTokenExchangePath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	var tokenSet credentials.CreLoginTokenSet
	if err := json.Unmarshal(body, &tokenSet); err != nil {
		return nil, fmt.Errorf("unmarshal token set: %w", err)
	}
	return &tokenSet, nil
}

func saveCredentials(tokenSet *credentials.CreLoginTokenSet) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, credentials.ConfigDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	path := filepath.Join(dir, credentials.ConfigFile)
	data, err := yaml.Marshal(tokenSet)
	if err != nil {
		return fmt.Errorf("marshal token set: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	return os.Rename(tmp, path)
}

func openBrowser(urlStr string) error {
	switch rt.GOOS {
	case "darwin":
		return exec.Command("open", urlStr).Start()
	case "linux":
		return exec.Command("xdg-open", urlStr).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", rt.GOOS)
	}
}
