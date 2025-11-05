package login

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	rt "runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/profiles"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

var (
	httpClient  = &http.Client{Timeout: 10 * time.Second}
	errorPage   = "htmlPages/error.html"
	successPage = "htmlPages/success.html"
	stylePage   = "htmlPages/output.css"
)

//go:embed htmlPages/*.html
//go:embed htmlPages/*.css
var htmlFiles embed.FS

func New(runtimeCtx *runtime.Context) *cobra.Command {
	var profileName string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Start authentication flow",
		Long:  "Opens browser for user login and saves credentials.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeCtx, profileName)
			return h.execute()
		},
	}

	cmd.Flags().StringVar(&profileName, "profile", "", "Profile name for this login (defaults to organization name)")

	return cmd
}

type handler struct {
	environmentSet   *environments.EnvironmentSet
	log              *zerolog.Logger
	profileName      string
	lastPKCEVerifier string
	lastState        string
}

func newHandler(ctx *runtime.Context, profileName string) *handler {
	return &handler{
		log:            ctx.Logger,
		environmentSet: ctx.EnvironmentSet,
		profileName:    profileName,
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

	// Use profile-based saving
	profileMgr, err := profiles.New(h.log)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to initialize profile manager")
		return err
	}

	// Generate profile name
	profileName := h.profileName
	if profileName == "" {
		// Default to a name derived from current time or use a sensible default
		profileName = "default"
		if len(profileMgr.ListProfiles()) == 0 {
			// First login, use "default"
			profileName = "default"
		} else {
			// For subsequent logins without explicit name, use a timestamp-based name
			profileName = fmt.Sprintf("profile_%d", time.Now().Unix())
		}
	}

	profile := &profiles.Profile{
		Name:     profileName,
		Tokens:   tokenSet,
		AuthType: credentials.AuthTypeBearer,
	}

	if err := profileMgr.SaveProfile(profile); err != nil {
		h.log.Error().Err(err).Msg("failed to save profile")
		return err
	}

	// Also save in legacy format for backwards compatibility (if it's the active profile)
	if err := credentials.SaveCredentials(tokenSet); err != nil {
		h.log.Warn().Err(err).Msg("failed to save credentials in legacy format (non-critical)")
	}

	fmt.Printf("\nLogin completed successfully!\n")
	fmt.Printf("Profile '%s' is now active\n", profileName)
	fmt.Println("To get started, run: cre init")
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

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", err
	}
	h.lastPKCEVerifier = verifier
	h.lastState = randomState()

	authURL := h.buildAuthURL(challenge, h.lastState)
	fmt.Printf("Opening browser to %s\n", authURL)
	if err := openBrowser(authURL, rt.GOOS); err != nil {
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
	listener, err := net.Listen("tcp", constants.AuthListenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %w", constants.AuthListenAddr, err)
	}

	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}, listener, nil
}

func (h *handler) callbackHandler(codeCh chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st := r.URL.Query().Get("state"); st == "" || h.lastState == "" || st != h.lastState {
			h.log.Error().Msg("invalid state in response")
			h.serveEmbeddedHTML(w, errorPage, http.StatusBadRequest)
			return
		}
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

func (h *handler) exchangeCodeForTokens(ctx context.Context, code string) (*credentials.CreLoginTokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", h.environmentSet.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", constants.AuthRedirectURI)
	form.Set("code_verifier", h.lastPKCEVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.environmentSet.AuthBase+constants.AuthTokenPath, strings.NewReader(form.Encode()))
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

func openBrowser(urlStr string, goos string) error {
	switch goos {
	case "darwin":
		return exec.Command("open", urlStr).Start()
	case "linux":
		return exec.Command("xdg-open", urlStr).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", goos)
	}
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
