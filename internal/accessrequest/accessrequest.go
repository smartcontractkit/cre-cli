package accessrequest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/ui"
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
}

func NewRequester(creds *credentials.Credentials, log *zerolog.Logger) *Requester {
	return &Requester{
		credentials: creds,
		log:         log,
	}
}

func (r *Requester) PromptAndSubmitRequest() error {
	ui.Line()
	ui.Warning("Deployment access is not yet enabled for your organization.")
	ui.Line()

	var shouldRequest bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Request deployment access?").
				Value(&shouldRequest),
		),
	).WithTheme(ui.ChainlinkTheme())

	if err := confirmForm.Run(); err != nil {
		return fmt.Errorf("failed to get user confirmation: %w", err)
	}

	if !shouldRequest {
		ui.Line()
		ui.Dim("Access request canceled.")
		return nil
	}

	var useCase string
	inputForm := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Briefly describe your use case").
				Description("What are you building with CRE?").
				Value(&useCase).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("use case description is required")
					}
					return nil
				}),
		),
	).WithTheme(ui.ChainlinkTheme())

	if err := inputForm.Run(); err != nil {
		return fmt.Errorf("failed to read use case: %w", err)
	}

	ui.Line()
	spinner := ui.NewSpinner()
	spinner.Start("Submitting access request...")

	if err := r.SubmitAccessRequest(useCase); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to submit access request: %w", err)
	}

	spinner.Stop()
	ui.Line()
	ui.Success("Access request submitted successfully!")
	ui.Line()
	ui.Dim("Our team will review your request and get back to you shortly.")
	ui.Line()

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

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if r.credentials.Tokens == nil || r.credentials.Tokens.AccessToken == "" {
		return fmt.Errorf("no access token available - please run 'cre login' first")
	}
	token := r.credentials.Tokens.AccessToken

	r.log.Debug().
		Str("url", apiURL).
		Str("method", "POST").
		Msg("submitting access request")

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
