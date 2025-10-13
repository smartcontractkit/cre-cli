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
	VerificationStatusSuccessful = "VERIFICATION_SUCCESSFULL" //nolint:misspell // Intentional misspelling to match external API
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
		// Even if link shows as confirmed, verify processing is complete
		fmt.Println("Link confirmed, verifying linking process...")
		if err := h.waitForBackendLinkProcessing(ownerAddr); err != nil {
			return fmt.Errorf("failed to verify linking process: %w", err)
		}

		time.Sleep(2 * time.Second)
		return nil
	}

	fmt.Printf("Owner not linked. Attempting auto-link: owner=%s\n", ownerAddr.Hex())
	if err := h.tryAutoLink(); err != nil {
		return fmt.Errorf("auto-link attempt failed: %w", err)
	}

	fmt.Printf("Auto-link successful: owner=%s\n", ownerAddr.Hex())

	// Wait for linking process to complete
	if err := h.waitForBackendLinkProcessing(ownerAddr); err != nil {
		return fmt.Errorf("linking process failed: %w", err)
	}

	time.Sleep(2 * time.Second)

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
		fmt.Printf("MSIG owner already linked. Continuing deploy: owner=%s\n", ownerAddr.Hex())
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
		WorkflowOwnerLabel:              "",
	}

	return linkkey.Exec(rtx, lkInputs)
}

// checkLinkStatusViaGraphQL checks if the owner is linked and verified by querying the service
func (h *handler) checkLinkStatusViaGraphQL(ownerAddr common.Address) (bool, error) {
	const query = `
	query {
		listWorkflowOwners(filters: { linkStatus: LINKED_ONLY }) {
			linkedOwners {
				workflowOwnerAddress
				verificationStatus
			}
		}
	}`

	req := graphql.NewRequest(query)
	var resp struct {
		ListWorkflowOwners struct {
			LinkedOwners []struct {
				WorkflowOwnerAddress string `json:"workflowOwnerAddress"`
				VerificationStatus   string `json:"verificationStatus"`
			} `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}

	gql := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	if err := gql.Execute(context.Background(), req, &resp); err != nil {
		return false, fmt.Errorf("GraphQL query failed: %w", err)
	}

	ownerHex := strings.ToLower(ownerAddr.Hex())
	for _, linkedOwner := range resp.ListWorkflowOwners.LinkedOwners {
		if strings.ToLower(linkedOwner.WorkflowOwnerAddress) == ownerHex {
			// Check if verification status is successful
			if linkedOwner.VerificationStatus == VerificationStatusSuccessful {
				return true, nil
			}
		}
	}

	return false, nil
}

// waitForBackendLinkProcessing polls the service until the link is processed
func (h *handler) waitForBackendLinkProcessing(ownerAddr common.Address) error {
	fmt.Printf("Waiting for linking process to complete: owner=%s\n", ownerAddr.Hex())

	err := retry.Do(
		func() error {
			linked, err := h.checkLinkStatusViaGraphQL(ownerAddr)
			if err != nil {
				h.log.Warn().Err(err).Msg("Failed to check link status")
				return err // Return error to trigger retry
			}
			if !linked {
				return fmt.Errorf("owner not yet linked and verified")
			}
			return nil // Success - owner is linked and verified
		},
		retry.Attempts(5),
		retry.Delay(3*time.Second),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			h.log.Debug().Uint("attempt", n+1).Uint("maxAttempts", 5).Err(err).Msg("Retrying link status check")
			fmt.Printf("Waiting for linking process... (attempt %d/%d)\n", n+1, 5)
		}),
	)

	if err != nil {
		return fmt.Errorf("linking process timeout after 10 attempts: %w", err)
	}

	fmt.Printf("Linking process confirmed: owner=%s\n", ownerAddr.Hex())
	return nil
}
