package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/machinebox/graphql"

	linkkey "github.com/smartcontractkit/cre-cli/cmd/account/link_key"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
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

	// Verify link with retry (blockchain state might take time to propagate)
	const maxRetries = 5
	const retryDelay = 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		if linked, err = h.wrc.IsOwnerLinked(ownerAddr); err != nil {
			return fmt.Errorf("linked via auto-link, but failed to verify link status: %w", err)
		} else if linked {
			fmt.Printf("Auto-link successful: owner=%s\n", ownerAddr.Hex())
			break
		}

		if i < maxRetries-1 {
			fmt.Printf("Waiting for blockchain state to propagate... (attempt %d/%d)\n", i+1, maxRetries)
			time.Sleep(retryDelay)
		} else {
			return fmt.Errorf("auto-link executed but owner still not linked after %d attempts", maxRetries)
		}
	}

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

// checkLinkStatusViaGraphQL checks if the owner is linked by querying the service
func (h *handler) checkLinkStatusViaGraphQL(ownerAddr common.Address) (bool, error) {
	const query = `
	query {
		listWorkflowOwners(filters: { linkStatus: LINKED_ONLY }) {
			linkedOwners {
				workflowOwnerAddress
			}
		}
	}`

	req := graphql.NewRequest(query)
	var resp struct {
		ListWorkflowOwners struct {
			LinkedOwners []struct {
				WorkflowOwnerAddress string `json:"workflowOwnerAddress"`
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
			return true, nil
		}
	}

	return false, nil
}

// waitForBackendLinkProcessing polls the service until the link is processed
func (h *handler) waitForBackendLinkProcessing(ownerAddr common.Address) error {
	const maxRetries = 5
	const retryInterval = 1 * time.Second

	fmt.Printf("Waiting for linking process to complete: owner=%s\n", ownerAddr.Hex())

	for i := 0; i < maxRetries; i++ {
		linked, err := h.checkLinkStatusViaGraphQL(ownerAddr)
		if err != nil {
			h.log.Warn().Err(err).Int("attempt", i+1).Int("maxRetries", maxRetries).Msg("Failed to check link status")
			// Continue retrying on errors as they might be temporary
		} else if linked {
			fmt.Printf("Linking process confirmed: owner=%s (attempt %d/%d)\n", ownerAddr.Hex(), i+1, maxRetries)
			return nil
		}

		if i < maxRetries-1 { // Don't print on the last attempt
			fmt.Printf("Waiting for linking process... (attempt %d/%d)\n", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("linking process timeout after %d attempts (waited %v)", maxRetries, time.Duration(maxRetries)*retryInterval)
}
