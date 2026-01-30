package accessrequest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
)

const (
	EnvVarAccessRequestURL = "CRE_ACCESS_REQUEST_URL"
)

type AccessRequest struct {
	UseCase string `json:"useCase"`
}

type Requester struct {
	credentials *credentials.Credentials
	log         *zerolog.Logger
	stdin       io.Reader
}

func NewRequester(creds *credentials.Credentials, log *zerolog.Logger, stdin io.Reader) *Requester {
	return &Requester{
		credentials: creds,
		log:         log,
		stdin:       stdin,
	}
}

func (r *Requester) PromptAndSubmitRequest() error {
	fmt.Println("")
	fmt.Println("Deployment access is not yet enabled for your organization.")
	fmt.Println("")

	shouldRequest, err := prompt.YesNoPrompt(r.stdin, "Request deployment access?")
	if err != nil {
		return fmt.Errorf("failed to get user confirmation: %w", err)
	}

	if !shouldRequest {
		fmt.Println("")
		fmt.Println("Access request canceled.")
		return nil
	}

	fmt.Println("")
	fmt.Println("Briefly describe your use case (what are you building with CRE?):")
	fmt.Print("> ")

	reader := bufio.NewReader(r.stdin)
	useCase, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read use case: %w", err)
	}
	useCase = strings.TrimSpace(useCase)

	if useCase == "" {
		return fmt.Errorf("use case description is required")
	}

	fmt.Println("")
	fmt.Println("Submitting access request...")

	if err := r.SubmitAccessRequest(useCase); err != nil {
		return fmt.Errorf("failed to submit access request: %w", err)
	}

	fmt.Println("")
	fmt.Println("Access request submitted successfully!")
	fmt.Println("")
	fmt.Println("Our team will review your request and get back to you shortly.")
	fmt.Println("")

	return nil
}

func (r *Requester) SubmitAccessRequest(useCase string) error {
	apiURL := os.Getenv(EnvVarAccessRequestURL)
	if apiURL == "" {
		return fmt.Errorf("access request API URL not configured (set %s environment variable)", EnvVarAccessRequestURL)
	}

	reqBody := AccessRequest{
		UseCase: useCase,
	}

	jsonBody, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if r.credentials.Tokens == nil || r.credentials.Tokens.AccessToken == "" {
		return fmt.Errorf("no access token available - please run 'cre login' first")
	}
	token := r.credentials.Tokens.AccessToken

	fmt.Println("")
	fmt.Println("Request Details:")
	fmt.Println("----------------")
	fmt.Printf("URL: %s\n", apiURL)
	fmt.Printf("Method: POST\n")
	fmt.Println("Headers:")
	fmt.Println("  Content-Type: application/json")
	fmt.Printf("  Authorization: Bearer %s\n", token)
	fmt.Println("Body:")
	fmt.Println(string(jsonBody))
	fmt.Println("----------------")

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("access request API returned status %d", resp.StatusCode)
	}

	return nil
}
