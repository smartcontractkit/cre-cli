package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	httptypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http"
)

type TriggerInput []byte

func (t *TriggerInput) UnmarshalJSON(bytes []byte) error {
	*t = bytes
	return nil
}

type BaseJsonRpc struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      string `json:"id"`
	Method  string `json:"method"`
}

type JsonRpcRequest struct {
	BaseJsonRpc
	Params struct {
		Input    TriggerInput `json:"input"`
		Workflow struct {
			WorkflowID string `json:"workflowID"`
		} `json:"workflow"`
	} `json:"params"`
}

type JsonRpcResponse struct {
	BaseJsonRpc
	Result struct {
		WorkflowID          string `json:"workflow_id"`
		WorkflowExecutionID string `json:"workflow_execution_id"`
		Status              string `json:"status"`
	} `json:"result"`
}

type GatewayConfig struct {
	Port    uint16
	Timeout time.Duration
}

func foo(ctx context.Context, config GatewayConfig) (*httptypedapi.Payload, error) {
	payloadCh := make(chan *httptypedapi.Payload, 1)
	defer close(payloadCh)

	errorCh := make(chan error, 1)
	defer close(errorCh)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		input, err := bar(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("error processing request: %v", err), http.StatusBadRequest)
			return
		}

		payloadCh <- &httptypedapi.Payload{
			Input: input.Params.Input,
		}
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}
	defer server.Close()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorCh <- err
		}
	}()

	select {
	case payload := <-payloadCh:
		return payload, nil
	case err := <-errorCh:
		return nil, err
	case <-time.After(config.Timeout):
		return nil, errors.New("timeout waiting for payload")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func bar(req *http.Request) (*JsonRpcRequest, error) {
	if req.Method != http.MethodPost {
		return nil, errors.New("gateway expects POST request")
	}

	authHeader := req.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return nil, errors.New("authorization header is missing")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	if err := baz(authHeader, body); err != nil {
		return nil, err
	}

	var input JsonRpcRequest
	if err := json.Unmarshal(body, &input); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	return &input, nil
}

func baz(header string, body []byte) error {
	header = strings.TrimSpace(header)
	if !strings.HasSuffix(header, "Bearer ") {
		return errors.New("invalid header")
	}
	jwt := header[len("Bearer "):]
	tokenParts := strings.Split(jwt, ".")
	if len(tokenParts) != 3 {
		return errors.New("invalid header")
	}

	jwtHeader, jwtPayload, jwtSignature := tokenParts[0], tokenParts[1], tokenParts[2]
	if err := validateJWTHeader(jwtHeader); err != nil {
		return err
	}

	return nil
}

func validateJWTHeader(header string) error {
	var values map[string]string
	if err := json.Unmarshal([]byte(header), &values); err != nil {
		return err
	}
	if values["alg"] != "ETH" {
		return errors.New("invalid algorithm")
	}
	if values["typ"] != "JWT" {
		return errors.New("invalid type")
	}
	return nil
}
