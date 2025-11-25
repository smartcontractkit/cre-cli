package credentials

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestNew_Default(t *testing.T) {
	t.Setenv(CreApiKeyVar, "")
	t.Setenv("HOME", t.TempDir())
	logger := testutil.NewTestLogger()

	_, err := New(logger)
	if err == nil || err.Error() != "you are not logged in, run cre login and try again" {
		t.Fatalf("expected error %q, got %v", "you are not logged in, run cre login and try again", err)
	}
}

func TestNew_WithEnvAPIKey(t *testing.T) {
	t.Setenv(CreApiKeyVar, "env-key")
	t.Setenv("HOME", t.TempDir())
	logger := testutil.NewTestLogger()

	cfg, err := New(logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("expected APIKey %q, got %q", "env-key", cfg.APIKey)
	}
	if cfg.AuthType != AuthTypeApiKey {
		t.Errorf("expected AuthType %q, got %q", AuthTypeApiKey, cfg.AuthType)
	}
}
func TestNew_WithConfigFile(t *testing.T) {
	t.Setenv(CreApiKeyVar, "")
	tDir := t.TempDir()
	t.Setenv("HOME", tDir)

	dir := filepath.Join(tDir, ConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	file := filepath.Join(dir, ConfigFile)
	content := `AccessToken: "file-token"
IDToken: "id-token"
RefreshToken: "refresh-token"
ExpiresIn:  99
TokenType:  "file-type"
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	logger := testutil.NewTestLogger()

	cfg, err := New(logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := cfg.Tokens.AccessToken; got != "file-token" {
		t.Errorf("expected AccessToken %q, got %q", "file-token", got)
	}
	if got := cfg.Tokens.IDToken; got != "id-token" {
		t.Errorf("expected IDToken %q, got %q", "id-token", got)
	}
	if got := cfg.Tokens.RefreshToken; got != "refresh-token" {
		t.Errorf("expected RefreshToken %q, got %q", "refresh-token", got)
	}
	if got := cfg.Tokens.ExpiresIn; got != 99 {
		t.Errorf("expected ExpiresIn %d, got %d", 99, got)
	}
	if got := cfg.Tokens.TokenType; got != "file-type" {
		t.Errorf("expected TokenType %q, got %q", "file-type", got)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
	if cfg.AuthType != AuthTypeBearer {
		t.Errorf("expected AuthType %q, got %q", AuthTypeBearer, cfg.AuthType)
	}
}

// Helper function to create a JWT token with custom claims
func createTestJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()

	// JWT header (doesn't matter for our tests)
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// JWT payload with claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("failed to marshal claims: %v", err)
	}
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// JWT signature (doesn't need to be valid for our tests)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return headerEncoded + "." + claimsEncoded + "." + signature
}

func TestCheckIsUngatedOrganization_APIKey(t *testing.T) {
	logger := testutil.NewTestLogger()
	creds := &Credentials{
		AuthType: AuthTypeApiKey,
		APIKey:   "test-api-key",
		log:      logger,
	}

	err := creds.CheckIsUngatedOrganization()
	if err != nil {
		t.Errorf("expected no error for API key auth, got: %v", err)
	}
}

func TestCheckIsUngatedOrganization_JWTWithFullAccess(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
	}{
		{
			name:      "production namespace",
			namespace: "https://api.cre.chain.link/",
		},
		{
			name:      "staging namespace",
			namespace: "https://graphql.cre.stage.internal.cldev.sh/",
		},
		{
			name:      "dev namespace",
			namespace: "https://graphql.cre.dev.internal.cldev.sh/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.NewTestLogger()

			claims := map[string]interface{}{
				"sub":                                "user123",
				"org_id":                             "org456",
				tc.namespace + "organization_status": "FULL_ACCESS",
				tc.namespace + "email":               "test@example.com",
				tc.namespace + "organization_roles":  "ROOT",
			}

			token := createTestJWT(t, claims)

			creds := &Credentials{
				AuthType: AuthTypeBearer,
				Tokens: &CreLoginTokenSet{
					AccessToken: token,
				},
				log: logger,
			}

			err := creds.CheckIsUngatedOrganization()
			if err != nil {
				t.Errorf("expected no error for FULL_ACCESS organization, got: %v", err)
			}
		})
	}
}

func TestCheckIsUngatedOrganization_JWTWithMissingClaim(t *testing.T) {
	logger := testutil.NewTestLogger()

	claims := map[string]interface{}{
		"sub":                              "user123",
		"org_id":                           "org456",
		"https://api.cre.chain.link/email": "test@example.com",
		"https://api.cre.chain.link/organization_roles": "ROOT",
		// organization_status claim is missing
	}

	token := createTestJWT(t, claims)

	creds := &Credentials{
		AuthType: AuthTypeBearer,
		Tokens: &CreLoginTokenSet{
			AccessToken: token,
		},
		log: logger,
	}

	err := creds.CheckIsUngatedOrganization()
	if err == nil {
		t.Error("expected error for missing organization_status claim, got nil")
	}
	if !strings.Contains(err.Error(), "early access") {
		t.Errorf("expected early access error, got: %v", err)
	}
}

func TestCheckIsUngatedOrganization_JWTWithEmptyStatus(t *testing.T) {
	logger := testutil.NewTestLogger()

	claims := map[string]interface{}{
		"sub":    "user123",
		"org_id": "org456",
		"https://api.cre.chain.link/organization_status": "",
	}

	token := createTestJWT(t, claims)

	creds := &Credentials{
		AuthType: AuthTypeBearer,
		Tokens: &CreLoginTokenSet{
			AccessToken: token,
		},
		log: logger,
	}

	err := creds.CheckIsUngatedOrganization()
	if err == nil {
		t.Error("expected error for empty organization_status, got nil")
	}
	if !strings.Contains(err.Error(), "early access") {
		t.Errorf("expected early access error, got: %v", err)
	}
}

func TestCheckIsUngatedOrganization_JWTWithGatedStatus(t *testing.T) {
	logger := testutil.NewTestLogger()

	claims := map[string]interface{}{
		"sub":    "user123",
		"org_id": "org456",
		"https://api.cre.chain.link/organization_status": "GATED",
	}

	token := createTestJWT(t, claims)

	creds := &Credentials{
		AuthType: AuthTypeBearer,
		Tokens: &CreLoginTokenSet{
			AccessToken: token,
		},
		log: logger,
	}

	err := creds.CheckIsUngatedOrganization()
	if err == nil {
		t.Error("expected error for GATED organization, got nil")
	}
	if !strings.Contains(err.Error(), "early access") {
		t.Errorf("expected early access error, got: %v", err)
	}
}

func TestCheckIsUngatedOrganization_JWTWithRestrictedStatus(t *testing.T) {
	logger := testutil.NewTestLogger()

	claims := map[string]interface{}{
		"sub":    "user123",
		"org_id": "org456",
		"https://api.cre.chain.link/organization_status": "RESTRICTED",
	}

	token := createTestJWT(t, claims)

	creds := &Credentials{
		AuthType: AuthTypeBearer,
		Tokens: &CreLoginTokenSet{
			AccessToken: token,
		},
		log: logger,
	}

	err := creds.CheckIsUngatedOrganization()
	if err == nil {
		t.Error("expected error for RESTRICTED organization, got nil")
	}
	if !strings.Contains(err.Error(), "early access") {
		t.Errorf("expected early access error, got: %v", err)
	}
}

func TestCheckIsUngatedOrganization_InvalidJWTFormat(t *testing.T) {
	testCases := []struct {
		name  string
		token string
	}{
		{
			name:  "not enough parts",
			token: "header.payload",
		},
		{
			name:  "invalid base64",
			token: "invalid!@#.invalid!@#.invalid!@#",
		},
		{
			name:  "empty token",
			token: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.NewTestLogger()

			creds := &Credentials{
				AuthType: AuthTypeBearer,
				Tokens: &CreLoginTokenSet{
					AccessToken: tc.token,
				},
				log: logger,
			}

			err := creds.CheckIsUngatedOrganization()
			if err == nil {
				t.Error("expected error for invalid JWT format, got nil")
			}
		})
	}
}
