package accessrequest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
)

const (
	EnvVarZendeskUsername = "CRE_ZENDESK_USERNAME"
	EnvVarZendeskPassword = "CRE_ZENDESK_PASSWORD"

	zendeskAPIURL           = "https://chainlinklabs.zendesk.com/api/v2/tickets.json"
	zendeskBrandID          = "41986419936660"
	zendeskRequestTypeField = "41987045113748"
	zendeskRequestTypeValue = "cre_customer_deploy_access_request"
)

type UserInfo struct {
	Email          string
	Name           string
	OrganizationID string
}

type Requester struct {
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	log            *zerolog.Logger
	stdin          io.Reader
}

func NewRequester(creds *credentials.Credentials, envSet *environments.EnvironmentSet, log *zerolog.Logger, stdin io.Reader) *Requester {
	return &Requester{
		credentials:    creds,
		environmentSet: envSet,
		log:            log,
		stdin:          stdin,
	}
}

func (r *Requester) PromptAndSubmitRequest(ctx context.Context) error {
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

	user, err := r.FetchUserInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch user info: %w", err)
	}

	fmt.Println("")
	fmt.Println("Submitting access request...")

	if err := r.SubmitAccessRequest(user); err != nil {
		return fmt.Errorf("failed to submit access request: %w", err)
	}

	fmt.Println("")
	fmt.Println("Access request submitted successfully!")
	fmt.Println("")
	fmt.Println("Our team will review your request and get back to you shortly.")
	fmt.Println("You'll receive a confirmation email at: " + user.Email)
	fmt.Println("")

	return nil
}

func (r *Requester) FetchUserInfo(ctx context.Context) (*UserInfo, error) {
	query := `
	query GetAccountDetails {
		getAccountDetails {
			emailAddress
		}
		getOrganization {
			organizationId
		}
	}`

	client := graphqlclient.New(r.credentials, r.environmentSet, r.log)
	req := graphql.NewRequest(query)

	var resp struct {
		GetAccountDetails struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"getAccountDetails"`
		GetOrganization struct {
			OrganizationID string `json:"organizationId"`
		} `json:"getOrganization"`
	}

	if err := client.Execute(ctx, req, &resp); err != nil {
		return nil, fmt.Errorf("graphql request failed: %w", err)
	}

	return &UserInfo{
		Email:          resp.GetAccountDetails.EmailAddress,
		Name:           resp.GetAccountDetails.EmailAddress,
		OrganizationID: resp.GetOrganization.OrganizationID,
	}, nil
}

func (r *Requester) SubmitAccessRequest(user *UserInfo) error {
	username := os.Getenv(EnvVarZendeskUsername)
	password := os.Getenv(EnvVarZendeskPassword)

	if username == "" || password == "" {
		return fmt.Errorf("zendesk credentials not configured (set %s and %s environment variables)", EnvVarZendeskUsername, EnvVarZendeskPassword)
	}

	ticket := map[string]interface{}{
		"ticket": map[string]interface{}{
			"subject": "CRE Deployment Access Request",
			"comment": map[string]interface{}{
				"body": fmt.Sprintf("Deployment access request submitted via CRE CLI.\n\nOrganization ID: %s", user.OrganizationID),
			},
			"brand_id": zendeskBrandID,
			"custom_fields": []map[string]interface{}{
				{
					"id":    zendeskRequestTypeField,
					"value": zendeskRequestTypeValue,
				},
			},
			"requester": map[string]interface{}{
				"name":  user.Name,
				"email": user.Email,
			},
		},
	}

	body, err := json.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, zendeskAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+creds)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zendesk API returned status %d", resp.StatusCode)
	}

	return nil
}
