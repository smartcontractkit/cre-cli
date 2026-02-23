package accessrequest

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const requestDeploymentAccessMutation = `
mutation RequestDeploymentAccess($input: RequestDeploymentAccessInput!) {
  requestDeploymentAccess(input: $input) {
    success
    message
  }
}`

type Requester struct {
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	log            *zerolog.Logger
}

func NewRequester(creds *credentials.Credentials, environmentSet *environments.EnvironmentSet, log *zerolog.Logger) *Requester {
	return &Requester{
		credentials:    creds,
		environmentSet: environmentSet,
		log:            log,
	}
}

func (r *Requester) PromptAndSubmitRequest(ctx context.Context) error {
	ui.Line()
	ui.Warning("Deployment access is not yet enabled for your organization.")
	ui.Line()

	shouldRequest := true
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
				CharLimit(1500).
				Value(&useCase).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
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

	if err := r.SubmitAccessRequest(ctx, useCase); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to submit access request: %w", err)
	}

	spinner.Stop()
	ui.Line()
	ui.Success("Access request submitted successfully!")
	ui.Line()
	ui.Print("Our team will review your request and get back to you via email shortly.")
	ui.Line()

	return nil
}

func (r *Requester) SubmitAccessRequest(ctx context.Context, useCase string) error {
	client := graphqlclient.New(r.credentials, r.environmentSet, r.log)

	req := graphql.NewRequest(requestDeploymentAccessMutation)
	req.Var("input", map[string]any{
		"description": useCase + " (Request from CLI)",
	})

	var resp struct {
		RequestDeploymentAccess struct {
			Success bool    `json:"success"`
			Message *string `json:"message"`
		} `json:"requestDeploymentAccess"`
	}

	if err := client.Execute(ctx, req, &resp); err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	if !resp.RequestDeploymentAccess.Success {
		msg := "access request was not successful"
		if resp.RequestDeploymentAccess.Message != nil {
			msg = *resp.RequestDeploymentAccess.Message
		}
		return fmt.Errorf("request failed: %s", msg)
	}

	return nil
}
