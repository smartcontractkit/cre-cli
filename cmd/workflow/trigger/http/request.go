package http

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	gwtypes "github.com/smartcontractkit/chainlink-common/pkg/types/gateway"
	"github.com/smartcontractkit/chainlink/v2/core/utils"
)

// triggerParams is gateway.HTTPTriggerRequest without the server-assigned key. It keeps
// input bytes verbatim, whereas that type's MarshalJSON round-trips them through float64.
type triggerParams struct {
	Input    json.RawMessage          `json:"input"`
	Workflow gwtypes.WorkflowSelector `json:"workflow"`
}

// signedRequest is a `workflows.execute` JSON-RPC body plus the JWT that authorizes it.
type signedRequest struct {
	ID     string
	Body   []byte
	Token  string
	Signer common.Address
}

// newSignedRequest signs with the node's own JWT helper so the digest and ETH signature
// scheme cannot drift from what the Gateway verifies.
func newSignedRequest(key *ecdsa.PrivateKey, workflowID string, input json.RawMessage) (*signedRequest, error) {
	req := jsonrpc2.Request[triggerParams]{
		Version: jsonrpc2.JsonRpcVersion,
		ID:      uuid.NewString(),
		Method:  gwtypes.MethodWorkflowExecute,
		Params: &triggerParams{
			Input:    input,
			Workflow: gwtypes.WorkflowSelector{WorkflowID: workflowID},
		},
	}

	signer := crypto.PubkeyToAddress(key.PublicKey)
	unsigned, err := utils.CreateRequestJWT(req, utils.WithIssuer(signer.Hex()))
	if err != nil {
		return nil, fmt.Errorf("failed to build request JWT: %w", err)
	}
	token, err := unsigned.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request JWT: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode trigger request: %w", err)
	}

	return &signedRequest{ID: req.ID, Body: body, Token: token, Signer: signer}, nil
}
