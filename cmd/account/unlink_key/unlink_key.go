package unlink_key

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	environment = "PRODUCTION_TESTNET"
)

type Inputs struct {
	WorkflowOwner                   string `validate:"workflow_owner"`
	WorkflowRegistryContractAddress string `validate:"required"`
	SkipConfirmation                bool
}

type initiateUnlinkingResponse struct {
	OwnershipProofHash string   `json:"ownershipProofHash"`
	ValidUntil         string   `json:"validUntil"`
	Signature          string   `json:"signature"`
	ChainSelector      string   `json:"chainSelector"`
	ContractAddress    string   `json:"contractAddress"`
	TransactionData    string   `json:"transactionData"`
	FunctionSignature  string   `json:"functionSignature"`
	FunctionArgs       []string `json:"functionArgs"`
}

type handler struct {
	settings       *settings.Settings
	credentials    *credentials.Credentials
	clientFactory  client.Factory
	log            *zerolog.Logger
	stdin          io.Reader
	environmentSet *environments.EnvironmentSet
	validated      bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink-key",
		Short: "Unlink a public key address from your account",
		Long:  `Unlink a previously linked public key address from your account, performing any pre-unlink cleanup.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeContext, cmd.InOrStdin())
			in, err := h.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			if err := h.ValidateInputs(in); err != nil {
				return err
			}
			return h.Execute(in)
		},
	}
	settings.AddRawTxFlag(cmd)
	settings.AddSkipConfirmation(cmd)
	return cmd
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	return &handler{
		settings:       ctx.Settings,
		credentials:    ctx.Credentials,
		clientFactory:  ctx.ClientFactory,
		log:            ctx.Logger,
		environmentSet: ctx.EnvironmentSet,
		stdin:          stdin,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		WorkflowOwner:                   h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowRegistryContractAddress: h.environmentSet.WorkflowRegistryAddress,
		SkipConfirmation:                v.GetBool(settings.Flags.SkipConfirmation.Name),
	}, nil
}

func (h *handler) ValidateInputs(in Inputs) error {
	v, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("validator init: %w", err)
	}
	if err := v.Struct(in); err != nil {
		return v.ParseValidationErrors(err)
	}
	h.validated = true
	return nil
}

func (h *handler) Execute(in Inputs) error {
	if !h.validated {
		return fmt.Errorf("inputs not validated")
	}

	fmt.Printf("Starting unlinking: owner=%s\n", in.WorkflowOwner)

	// Check if confirmation should be skipped
	if !in.SkipConfirmation {
		deleteWorkflows, err := prompt.YesNoPrompt(
			h.stdin,
			"Warning: Unlink is a destructive action that will wipe out all workflows registered under your owner address. Do you wish to proceed?",
		)
		if err != nil {
			return err
		}
		if !deleteWorkflows {
			return fmt.Errorf("unlinking aborted by user")
		}
	}

	resp, err := h.callInitiateUnlinking(context.Background(), in)
	if err != nil {
		return err
	}

	prettyResp, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal linking response")
		return err
	}

	h.log.Debug().Msg("\nRaw linking response payload:\n\n" + string(prettyResp))

	if in.WorkflowRegistryContractAddress == resp.ContractAddress {
		fmt.Println("Contract address validation passed")
	} else {
		return fmt.Errorf("contract address validation failed")
	}

	return h.unlinkOwner(in.WorkflowOwner, resp)
}

func (h *handler) callInitiateUnlinking(ctx context.Context, in Inputs) (initiateUnlinkingResponse, error) {
	const mutation = `
mutation InitiateUnlinking($request: InitiateUnlinkingRequest!) {
  initiateUnlinking(request: $request) {
    ownershipProofHash
    validUntil
    signature
    chainSelector
    contractAddress
    transactionData
    functionSignature
    functionArgs
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", map[string]any{
		"workflowOwnerAddress": in.WorkflowOwner,
		"environment":          environment,
	})
	req.Header.Set("Idempotency-Key", uuid.New().String())

	var container struct {
		InitiateUnlinking initiateUnlinkingResponse `json:"initiateUnlinking"`
	}
	if err := graphqlclient.New(h.credentials, h.environmentSet, h.log).Execute(ctx, req, &container); err != nil {
		return initiateUnlinkingResponse{}, fmt.Errorf("graphql failed: %w", err)
	}

	h.log.Debug().Interface("response", container).
		Msg("Received GraphQL response from initiate unlinking")

	return container.InitiateUnlinking, nil
}

func (h *handler) unlinkOwner(owner string, resp initiateUnlinkingResponse) error {
	expiresAt, err := time.Parse(time.RFC3339, resp.ValidUntil)
	if err != nil {
		return fmt.Errorf("invalid validUntil format: %w", err)
	}
	if time.Now().UTC().After(expiresAt) {
		return fmt.Errorf("the request has expired")
	}

	ts := big.NewInt(expiresAt.Unix())

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(resp.Signature, "0x"))
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("wrc init: %w", err)
	}
	addr := common.HexToAddress(owner)
	if err := wrc.CanUnlinkOwner(addr, ts, sigBytes); err != nil {
		return fmt.Errorf("unlink request verification failed: %w", err)
	}
	txOut, err := wrc.UnlinkOwner(addr, ts, sigBytes)
	if err != nil {
		return fmt.Errorf("UnlinkOwner failed: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		fmt.Printf("Transaction submitted: %s\n", txOut.Hash)
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)

	case client.Raw:
		selector, err := strconv.ParseUint(resp.ChainSelector, 10, 64)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to parse chain selector")
			return err
		}
		ChainName, err := settings.GetChainNameByChainSelector(selector)
		if err != nil {
			h.log.Error().Err(err).Uint64("selector", selector).Msg("failed to get chain name")
			return err
		}

		fmt.Println("")
		fmt.Println("Ownership unlinking initialized successfully!")
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("")
		fmt.Println("   1. Submit the following transaction on the target chain:")
		fmt.Println("")
		fmt.Printf("      Chain:            %s\n", ChainName)
		fmt.Printf("      Contract Address: %s\n", resp.ContractAddress)
		fmt.Println("")
		fmt.Println("   2. Use the following transaction data:")
		fmt.Println("")
		fmt.Printf("      %s\n", resp.TransactionData)
		fmt.Println("")
	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}

	fmt.Println("Unlinked successfully")
	return nil
}
