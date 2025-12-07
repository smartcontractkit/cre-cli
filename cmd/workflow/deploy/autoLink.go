package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/machinebox/graphql"

	linkkey "github.com/smartcontractkit/cre-cli/cmd/account/link_key"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const (
	VerificationStatusSuccessful = "VERIFICATION_STATUS_SUCCESSFULL" //nolint:misspell // Intentional misspelling to match external API
)

// ensureOwnerLinkedOrFail checks if the owner is linked and attempts auto-link if needed
func (h *handler) ensureOwnerLinkedOrFail() error {
	ownerAddr := common.HexToAddress(h.inputs.WorkflowOwner)

	linked, err := h.wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return fmt.Errorf("failed to check owner link status: %w", err)
	}

	fmt.Printf("Workflow owner link status: owner=%s, linked=%v\n", ownerAddr.Hex(), linked)

	if linked {
		// Owner is linked on contract, now verify it's linked to the current user's account
		linkedToCurrentUser, _, err := h.checkLinkStatusViaGraphQL(ownerAddr)
		if err != nil {
			return fmt.Errorf("failed to validate key ownership: %w", err)
		}

		if !linkedToCurrentUser {
			return fmt.Errorf("key %s is linked to another account. Please use a different owner address", ownerAddr.Hex())
		}

		fmt.Println("Key ownership verified")
		return nil
	}

	fmt.Printf("Owner not linked. Attempting auto-link: owner=%s\n", ownerAddr.Hex())
	if err := h.tryAutoLink(); err != nil {
		return fmt.Errorf("auto-link attempt failed: %w", err)
	}

	linkSuccessTime := time.Now()
	fmt.Printf("Auto-link successful: owner=%s\n", ownerAddr.Hex())
	fmt.Println("Note: Linking verification may take up to 60 seconds.")

	// Wait for linking process to complete
	if err := h.waitForBackendLinkProcessing(ownerAddr, linkSuccessTime); err != nil {
		return fmt.Errorf("linking process failed: %w", err)
	}

	return nil
}

// autoLinkMSIGAndExit handles MSIG auto-link and exits if manual intervention is needed
func (h *handler) autoLinkMSIGAndExit() (halt bool, err error) {
	ownerAddr := common.HexToAddress(h.inputs.WorkflowOwner)

	linked, err := h.wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return false, fmt.Errorf("failed to check owner link status: %w", err)
	}

	if linked {
		// Owner is linked on contract, now verify it's linked to the current user's account
		linkedToCurrentUser, _, err := h.checkLinkStatusViaGraphQL(ownerAddr)
		if err != nil {
			return false, fmt.Errorf("failed to validate MSIG key ownership: %w", err)
		}

		if !linkedToCurrentUser {
			return false, fmt.Errorf("MSIG key %s is linked to another account. Please use a different owner address", ownerAddr.Hex())
		}

		fmt.Printf("MSIG key ownership verified. Continuing deploy: owner=%s\n", ownerAddr.Hex())
		return false, nil
	}

	fmt.Printf("MSIG workflow owner link status: owner=%s, linked=%v\n", ownerAddr.Hex(), linked)
	fmt.Printf("MSIG owner: attempting auto-link... owner=%s\n", ownerAddr.Hex())

	if err := h.tryAutoLink(); err != nil {
		return false, fmt.Errorf("MSIG auto-link attempt failed: %w", err)
	}

	fmt.Println("MSIG auto-link initiated. Halting deploy. Submit the multisig transaction, then re-run deploy.")
	return true, nil
}

// tryAutoLink executes the auto-link process using the link-key command
func (h *handler) tryAutoLink() error {
	rtx := &runtime.Context{
		Settings:       h.settings,
		Credentials:    h.credentials,
		ClientFactory:  h.clientFactory,
		Logger:         h.log,
		EnvironmentSet: h.environmentSet,
	}

	lkInputs := linkkey.Inputs{
		WorkflowOwner:                   h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowRegistryContractAddress: h.inputs.WorkflowRegistryContractAddress,
		WorkflowOwnerLabel:              h.inputs.OwnerLabel,
	}

	return linkkey.Exec(rtx, lkInputs)
}

// checkLinkStatusViaGraphQL checks if the owner is linked and verified by querying the service
func (h *handler) checkLinkStatusViaGraphQL(ownerAddr common.Address) (bool, *time.Time, error) {
	const query = `
	query {
		listWorkflowOwners(filters: { linkStatus: LINKED_ONLY }) {
			linkedOwners {
				workflowOwnerAddress
				verificationStatus
				verifiedAt
			}
		}
	}`

	req := graphql.NewRequest(query)
	var resp struct {
		ListWorkflowOwners struct {
			LinkedOwners []struct {
				WorkflowOwnerAddress string     `json:"workflowOwnerAddress"`
				VerificationStatus   string     `json:"verificationStatus"`
				VerifiedAt           *time.Time `json:"verifiedAt"`
			} `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}

	gql := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	if err := gql.Execute(context.Background(), req, &resp); err != nil {
		return false, nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	ownerHex := strings.ToLower(ownerAddr.Hex())
	for _, linkedOwner := range resp.ListWorkflowOwners.LinkedOwners {
		if strings.ToLower(linkedOwner.WorkflowOwnerAddress) == ownerHex {
			// Check if verification status is successful
			if linkedOwner.VerificationStatus == VerificationStatusSuccessful {
				h.log.Debug().
					Str("ownerAddress", linkedOwner.WorkflowOwnerAddress).
					Str("verificationStatus", linkedOwner.VerificationStatus).
					Msg("Owner found and verified")
				return true, linkedOwner.VerifiedAt, nil
			}
			h.log.Debug().
				Str("ownerAddress", linkedOwner.WorkflowOwnerAddress).
				Str("verificationStatus", linkedOwner.VerificationStatus).
				Str("expectedStatus", VerificationStatusSuccessful).
				Msg("Owner found but verification status not successful")
			return false, nil, nil
		}
	}

	h.log.Debug().
		Str("ownerAddress", ownerAddr.Hex()).
		Msg("Owner not found in linked owners list")

	return false, nil, nil
}

// waitForBackendLinkProcessing polls the service until the link is processed
func (h *handler) waitForBackendLinkProcessing(ownerAddr common.Address, linkSuccessTime time.Time) error {
	const maxAttempts = 5
	const retryDelay = 3 * time.Second

	fmt.Printf("Waiting for linking process to complete: owner=%s\n", ownerAddr.Hex())

	var verifiedAt *time.Time
	err := retry.Do(
		func() error {
			linked, vAt, err := h.checkLinkStatusViaGraphQL(ownerAddr)
			if err != nil {
				h.log.Warn().Err(err).Msg("Failed to check link status")
				return err // Return error to trigger retry
			}
			if !linked {
				return fmt.Errorf("owner not yet linked and verified")
			}
			verifiedAt = vAt
			return nil // Success - owner is linked and verified
		},
		retry.Attempts(maxAttempts),
		retry.Delay(retryDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			h.log.Debug().Uint("attempt", n+1).Uint("maxAttempts", maxAttempts).Err(err).Msg("Retrying link status check")
			fmt.Printf("Waiting for linking process... (attempt %d/%d)\n", n+1, maxAttempts)
		}),
	)

	if err != nil {
		return fmt.Errorf("linking process timeout after %d attempts: %w", maxAttempts, err)
	}

	// Calculate and log the time between link success and verification
	if verifiedAt != nil {
		processingTime := verifiedAt.Sub(linkSuccessTime)
		h.log.Info().
			Str("owner", ownerAddr.Hex()).
			Time("linkSuccessTime", linkSuccessTime).
			Time("verifiedAt", *verifiedAt).
			Dur("processingTime", processingTime).
			Msgf("Link verification processing time: %.2f seconds", processingTime.Seconds())
		fmt.Printf("Backend processing time: %.2f seconds (from transaction success to verified status)\n", processingTime.Seconds())
	}

	fmt.Printf("Linking process confirmed: owner=%s\n", ownerAddr.Hex())
	return nil
}
