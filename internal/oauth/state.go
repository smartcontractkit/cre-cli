package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
)

// RandomState returns a URL-safe random string suitable for OAuth "state".
func RandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizeURLWithState sets or replaces the OAuth "state" query parameter on an authorize URL.
func AuthorizeURLWithState(rawURL, state string) (string, error) {
	if state == "" {
		return "", fmt.Errorf("oauth: state must not be empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// StateFromAuthorizeURL returns the OAuth "state" query parameter from an authorize URL, if present.
func StateFromAuthorizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.Query().Get("state"), nil
}

// ClientIDFromAuthorizeURL returns the "client_id" query parameter from an authorize URL (if present).
// Token exchange must use the same client_id the IdP bound to the authorization code.
func ClientIDFromAuthorizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.Query().Get("client_id"), nil
}

// OAuthServerBaseFromAuthorizeURL returns the authorization server origin (scheme + host) for the
// given authorize URL. The token endpoint must be on the same host that issued the authorization code.
func OAuthServerBaseFromAuthorizeURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("authorize URL missing scheme or host")
	}
	return u.Scheme + "://" + u.Host, nil
}
