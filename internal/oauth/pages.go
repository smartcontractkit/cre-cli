package oauth

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

const (
	PageError          = "htmlPages/error.html"
	PageSuccess        = "htmlPages/success.html"
	PageSecretsSuccess = "htmlPages/secrets_success.html"
	PageSecretsError   = "htmlPages/secrets_error.html"
	PageWaiting        = "htmlPages/waiting.html"
	StylePage          = "htmlPages/output.css"
)

//go:embed htmlPages/*.html
//go:embed htmlPages/*.css
var htmlFiles embed.FS

// ServeEmbeddedHTML serves an embedded HTML page with inline CSS.
func ServeEmbeddedHTML(log *zerolog.Logger, w http.ResponseWriter, filePath string, status int) {
	htmlContent, err := htmlFiles.ReadFile(filePath)
	if err != nil {
		log.Error().Err(err).Str("file", filePath).Msg("failed to read embedded HTML file")
		sendHTTPError(w)
		return
	}

	cssContent, err := htmlFiles.ReadFile(StylePage)
	if err != nil {
		log.Error().Err(err).Str("file", StylePage).Msg("failed to read embedded CSS file")
		sendHTTPError(w)
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
		log.Error().Err(err).Msg("failed to write HTML response")
	}
}

// ServeWaitingPage serves the waiting page with the redirect URL injected.
func ServeWaitingPage(log *zerolog.Logger, w http.ResponseWriter, redirectURL string) {
	htmlContent, err := htmlFiles.ReadFile(PageWaiting)
	if err != nil {
		log.Error().Err(err).Str("file", PageWaiting).Msg("failed to read waiting page HTML file")
		sendHTTPError(w)
		return
	}

	cssContent, err := htmlFiles.ReadFile(StylePage)
	if err != nil {
		log.Error().Err(err).Str("file", StylePage).Msg("failed to read embedded CSS file")
		sendHTTPError(w)
		return
	}

	modified := strings.Replace(
		string(htmlContent),
		`<link rel="stylesheet" href="./output.css" />`,
		fmt.Sprintf("<style>%s</style>", string(cssContent)),
		1,
	)

	modified = strings.Replace(modified, "{{REDIRECT_URL}}", redirectURL, 1)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(modified)); err != nil {
		log.Error().Err(err).Msg("failed to write waiting page response")
	}
}

func sendHTTPError(w http.ResponseWriter) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
