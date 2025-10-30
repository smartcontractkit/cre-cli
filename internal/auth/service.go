package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type OAuthService struct {
	environmentSet *environments.EnvironmentSet
}

func NewOAuthService(environmentSet *environments.EnvironmentSet) *OAuthService {
	return &OAuthService{environmentSet: environmentSet}
}

func (s *OAuthService) buildURL(path string) string {
	return s.environmentSet.AuthBase + path
}

func (s *OAuthService) RefreshToken(ctx context.Context, oldTokenSet *credentials.CreLoginTokenSet) (*credentials.CreLoginTokenSet, error) {
	if oldTokenSet.RefreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", s.environmentSet.ClientID)
	form.Set("refresh_token", oldTokenSet.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.buildURL(constants.AuthTokenPath), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("auth response: unauthorized (401) - you have been logged out. " +
			"Please login using `cre login` and retry your command")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth response: %s", resp.Status)
	}

	var tr struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}

	newRefresh := oldTokenSet.RefreshToken
	if tr.RefreshToken != "" {
		newRefresh = tr.RefreshToken
	}

	return &credentials.CreLoginTokenSet{
		AccessToken:  tr.AccessToken,
		IDToken:      tr.IDToken,
		RefreshToken: newRefresh,
		ExpiresIn:    tr.ExpiresIn,
		TokenType:    tr.TokenType,
	}, nil
}

func (s *OAuthService) RevokeToken(ctx context.Context, token string) error {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", s.environmentSet.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.buildURL(constants.AuthRevokePath), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("revocation failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revocation failed: %s", resp.Status)
	}
	return nil
}
