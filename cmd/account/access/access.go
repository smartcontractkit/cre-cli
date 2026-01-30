package access

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
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const (
	// Environment variables for Zendesk credentials
	EnvVarZendeskUsername = "CRE_ZENDESK_USERNAME"
	EnvVarZendeskPassword = "CRE_ZENDESK_PASSWORD"

	// Zendesk configuration
	zendeskAPIURL           = "https://chainlinklabs.zendesk.com/api/v2/tickets.json"
	zendeskBrandID          = "41986419936660"
	zendeskRequestTypeField = "41987045113748"
	zendeskRequestTypeValue = "cre_customer_deploy_access_request"
)

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Check or request deployment access",
		Long:  "Check your deployment access status or request access to deploy workflows.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := NewHandler(runtimeCtx, cmd.InOrStdin())
			return h.Execute(cmd.Context())
		},
	}

	return cmd
}

type Handler struct {
	log            *zerolog.Logger
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	stdin          io.Reader
}

func NewHandler(ctx *runtime.Context, stdin io.Reader) *Handler {
	return &Handler{
		log:            ctx.Logger,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
		stdin:          stdin,
	}
}

type userInfo struct {
	Email          string
	Name           string
	OrganizationID string
}

func (h *Handler) Execute(ctx context.Context) error {
	// Get deployment access status
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		return fmt.Errorf("failed to check deployment access: %w", err)
	}

	if deployAccess.HasAccess {
		fmt.Println("")
		fmt.Println("You have deployment access enabled for your organization.")
		fmt.Println("")
		fmt.Println("You're all set to deploy workflows. Get started with:")
		fmt.Println("")
		fmt.Println("  cre workflow deploy")
		fmt.Println("")
		fmt.Println("For more information, run 'cre workflow deploy --help'")
		fmt.Println("")
		return nil
	}

	// User doesn't have access - prompt to submit request
	fmt.Println("")
	fmt.Println("Deployment access is not yet enabled for your organization.")
	fmt.Println("")

	// Ask user if they want to request access
	shouldRequest, err := prompt.YesNoPrompt(h.stdin, "Request deployment access?")
	if err != nil {
		return fmt.Errorf("failed to get user confirmation: %w", err)
	}

	if !shouldRequest {
		fmt.Println("")
		fmt.Println("Access request canceled.")
		return nil
	}

	// Fetch user info for the request
	user, err := h.fetchUserInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch user info: %w", err)
	}

	fmt.Println("")
	fmt.Println("Submitting access request...")

	if err := h.submitAccessRequest(user); err != nil {
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

func (h *Handler) fetchUserInfo(ctx context.Context) (*userInfo, error) {
	query := `
	query GetAccountDetails {
		getAccountDetails {
			emailAddress
		}
		getOrganization {
			organizationId
		}
	}`

	client := graphqlclient.New(h.credentials, h.environmentSet, h.log)
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

	// Use email as name since firstName/lastName are not available in the schema
	name := resp.GetAccountDetails.EmailAddress

	return &userInfo{
		Email:          resp.GetAccountDetails.EmailAddress,
		Name:           name,
		OrganizationID: resp.GetOrganization.OrganizationID,
	}, nil
}

func (h *Handler) submitAccessRequest(user *userInfo) error {
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

	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+credentials)

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
