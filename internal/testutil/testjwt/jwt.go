package testjwt

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// CreateTestJWTWithClaims creates a JWT token with custom claims for testing.
// The signature is a dummy value.
func CreateTestJWTWithClaims(claims map[string]interface{}) string {
	// JWT header (doesn't matter for our tests)
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// JWT payload with claims
	claimsJSON, _ := json.Marshal(claims)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// JWT signature (doesn't need to be valid for our tests)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return headerEncoded + "." + claimsEncoded + "." + signature
}

// CreateTestJWT creates a JWT token with default claims for a given organization ID.
func CreateTestJWT(orgID string) string {
	return CreateTestJWTWithClaims(map[string]interface{}{
		"sub":                 "test-user",
		"org_id":              orgID,
		"organization_status": "FULL_ACCESS",
		"exp":                 time.Now().Add(2 * time.Hour).Unix(),
	})
}
