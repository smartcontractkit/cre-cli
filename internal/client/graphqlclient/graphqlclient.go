package graphqlclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/auth"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/profiles"
)

const bufferSeconds = 60

type Client struct {
	client *graphql.Client
	creds  *credentials.Credentials
	log    *zerolog.Logger
	auth   *auth.OAuthService
}

func New(creds *credentials.Credentials, environmentSet *environments.EnvironmentSet, l *zerolog.Logger) *Client {
	gqlClient := graphql.NewClient(environmentSet.GraphQLURL)
	gqlClient.Log = func(s string) {
		l.Debug().Str("client", "GraphQL").Msg(s)
	}

	return &Client{
		client: gqlClient,
		creds:  creds,
		log:    l,
		auth:   auth.NewOAuthService(environmentSet),
	}
}

func (c *Client) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	if c.creds == nil {
		return fmt.Errorf("credentials not provided")
	}
	req.Header.Set("User-Agent", "cre-cli")
	err := c.refreshTokenIfNeeded(ctx)
	if err != nil {
		return fmt.Errorf("token validity check failed: %w", err)
	}

	switch c.creds.AuthType {
	case credentials.AuthTypeApiKey:
		if c.creds.APIKey != "" {
			req.Header.Set("Authorization", "Apikey "+c.creds.APIKey)
		}
	default:
		if c.creds.Tokens != nil && c.creds.Tokens.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.creds.Tokens.AccessToken)
		}
	}
	return c.client.Run(ctx, req, resp)
}

func (c *Client) refreshTokens(ctx context.Context) error {
	if c.creds == nil || c.creds.Tokens == nil || c.creds.Tokens.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}
	c.log.Debug().Msg("refreshing tokens")
	newTokens, err := c.auth.RefreshToken(ctx, c.creds.Tokens)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}
	c.log.Debug().Msg("token refreshed")
	c.creds.Tokens = newTokens

	// Save to profile if credentials were loaded from a profile, otherwise use legacy format
	if c.creds.ProfileName != "" {
		// Profile-aware: save to profile system
		profileMgr, err := profiles.New(c.log)
		if err != nil {
			c.log.Warn().Err(err).Msg("failed to create profile manager for token refresh")
		} else {
			profile := profileMgr.GetProfile(c.creds.ProfileName)
			if profile != nil {
				profile.Tokens = newTokens
				if err := profileMgr.SaveProfile(profile); err != nil {
					c.log.Warn().Err(err).Msg("failed to save refreshed tokens to profile")
				} else {
					c.log.Debug().Msg("refreshed tokens saved to profile")
					return nil
				}
			}
		}
	}

	// Fallback to legacy format for backward compatibility or if profile save failed
	if err := credentials.SaveCredentials(newTokens); err != nil {
		c.log.Warn().Err(err).Msg("failed to save credentials in legacy format (non-critical)")
	} else {
		c.log.Debug().Msg("refreshed tokens saved to disk")
	}
	return nil
}

func (c *Client) refreshTokenIfNeeded(ctx context.Context) error {
	if c.creds == nil || c.creds.Tokens == nil || c.creds.Tokens.AccessToken == "" {
		return nil
	}
	parts := strings.Split(c.creds.Tokens.AccessToken, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid JWT token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	if time.Now().Unix() >= claims.Exp-bufferSeconds {
		c.log.Debug().Msg("token expired or approaching expiration, refreshing")
		return c.refreshTokens(ctx)
	}

	return nil
}
