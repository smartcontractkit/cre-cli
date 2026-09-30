package http

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/utils"
)

const testWorkflowID = "0x1010101010101010101010101010101010101010101010101010101010101010"

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return key
}

// decodeAsGateway mirrors how the Gateway decodes the wire body, so assertions run against
// what it would actually verify.
func decodeAsGateway(t *testing.T, signed *signedRequest) jsonrpc2.Request[json.RawMessage] {
	t.Helper()
	req, err := jsonrpc2.DecodeRequest[json.RawMessage](signed.Body, signed.Token)
	require.NoError(t, err)
	return req
}

func decodeSegment(t *testing.T, segment string, into any) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, into))
}

func TestNewSignedRequest_JWTRecoversToSignerAndDigestMatchesBody(t *testing.T) {
	key := newTestKey(t)
	signed, err := newSignedRequest(key, testWorkflowID, json.RawMessage(`{"orderId":42}`))
	require.NoError(t, err)

	req := decodeAsGateway(t, signed)
	claims, recovered, err := utils.VerifyRequestJWT(signed.Token, req)
	require.NoError(t, err)

	assert.Equal(t, crypto.PubkeyToAddress(key.PublicKey), recovered)
	assert.Equal(t, signed.Signer, recovered)
	assert.Equal(t, recovered.Hex(), claims.Issuer)

	digest, err := req.Digest()
	require.NoError(t, err)
	assert.Equal(t, "0x"+digest, claims.Digest)
}

func TestNewSignedRequest_JWTHeaderAndClaims(t *testing.T) {
	key := newTestKey(t)
	before := time.Now().Add(-time.Second)
	signed, err := newSignedRequest(key, testWorkflowID, json.RawMessage(`{}`))
	require.NoError(t, err)

	parts := strings.Split(signed.Token, ".")
	require.Len(t, parts, 3)

	var header map[string]string
	decodeSegment(t, parts[0], &header)
	assert.Equal(t, map[string]string{"alg": "ETH", "typ": "JWT"}, header)

	var claims utils.JWTClaims
	decodeSegment(t, parts[1], &claims)
	_, parseErr := uuid.Parse(claims.ID)
	assert.NoError(t, parseErr, "jti must be a UUID")
	assert.True(t, claims.IssuedAt.After(before))
	assert.Equal(t, 5*time.Minute, claims.ExpiresAt.Sub(claims.IssuedAt.Time), "lifetime must stay within the Gateway's maximum")
}

func TestNewSignedRequest_TamperedBodyFailsDigestCheck(t *testing.T) {
	key := newTestKey(t)
	signed, err := newSignedRequest(key, testWorkflowID, json.RawMessage(`{"orderId":42}`))
	require.NoError(t, err)

	tampered := strings.Replace(string(signed.Body), "42", "43", 1)
	require.NotEqual(t, string(signed.Body), tampered)
	req, err := jsonrpc2.DecodeRequest[json.RawMessage]([]byte(tampered), signed.Token)
	require.NoError(t, err)

	_, _, err = utils.VerifyRequestJWT(signed.Token, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match calculated request digest")
}

func TestNewSignedRequest_WireShape(t *testing.T) {
	signed, err := newSignedRequest(newTestKey(t), testWorkflowID, json.RawMessage(`{"a":[1,2]}`))
	require.NoError(t, err)

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(signed.Body, &wire))
	assert.JSONEq(t, `"2.0"`, string(wire["jsonrpc"]))
	assert.JSONEq(t, `"workflows.execute"`, string(wire["method"]))
	assert.JSONEq(t, `"`+signed.ID+`"`, string(wire["id"]))
	assert.NotContains(t, wire, "auth", "the JWT travels in the Authorization header, not the body")
	assert.JSONEq(t, `{"input":{"a":[1,2]},"workflow":{"workflowID":"`+testWorkflowID+`"}}`, string(wire["params"]))
	_, idErr := uuid.Parse(signed.ID)
	assert.NoError(t, idErr)
	assert.NotContains(t, signed.ID, "/", "the Gateway reserves '/' in request IDs")
}

func TestNewSignedRequest_KeepsInputNumbersVerbatim(t *testing.T) {
	signed, err := newSignedRequest(newTestKey(t), testWorkflowID, json.RawMessage(`{"amount":12345678901234567890}`))
	require.NoError(t, err)

	assert.Contains(t, string(signed.Body), `"amount":12345678901234567890`)
}

func TestNewSignedRequest_RequestIDAndJTIAreUniquePerCall(t *testing.T) {
	key := newTestKey(t)
	first, err := newSignedRequest(key, testWorkflowID, json.RawMessage(`{}`))
	require.NoError(t, err)
	second, err := newSignedRequest(key, testWorkflowID, json.RawMessage(`{}`))
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)

	jti := func(token string) string {
		var claims utils.JWTClaims
		decodeSegment(t, strings.Split(token, ".")[1], &claims)
		return claims.ID
	}
	assert.NotEqual(t, jti(first.Token), jti(second.Token), "a reused jti is rejected as a replay")
}
