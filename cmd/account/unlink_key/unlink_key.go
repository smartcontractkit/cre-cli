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
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
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

	h.log.Info().Str("owner", in.WorkflowOwner).Msg("Starting unlinking")

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

	resp, err := h.callInitiateUnlinking(context.Background(), in)
	if err != nil {
		return err
	}

	if in.WorkflowRegistryContractAddress == resp.ContractAddress {
		h.log.Info().Msg("Contract address validation passed")
	} else {
		return fmt.Errorf("contract address validation failed")
	}

	switch h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType {
	case constants.WorkflowOwnerTypeMSIG:
		return h.unlinkOwnerUsingMSIG(resp)
	default:
		return h.unlinkOwnerUsingEOA(in.WorkflowOwner, resp)
	}
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

	var container struct {
		InitiateUnlinking initiateUnlinkingResponse `json:"initiateUnlinking"`
	}
	if err := graphqlclient.New(h.credentials, h.environmentSet).Execute(ctx, req, &container); err != nil {
		return initiateUnlinkingResponse{}, fmt.Errorf("graphql failed: %w", err)
	}

	h.log.Debug().Interface("response", container).
		Msg("Received GraphQL response from initiate unlinking")

	return container.InitiateUnlinking, nil
}

func (h *handler) unlinkOwnerUsingEOA(owner string, resp initiateUnlinkingResponse) error {
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

	client, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("client init: %w", err)
	}
	addr := common.HexToAddress(owner)
	if err := client.CanUnlinkOwner(addr, ts, sigBytes); err != nil {
		return fmt.Errorf("unlink request verification failed: %w", err)
	}
	if err := client.UnlinkOwner(addr, ts, sigBytes); err != nil {
		return fmt.Errorf("UnlinkOwner failed: %w", err)
	}

	h.log.Info().Msg("Unlinked successfully")
	return nil
}

// TODO: Add support for output file
func (h *handler) unlinkOwnerUsingMSIG(resp initiateUnlinkingResponse) error {
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

	client, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("client init: %w", err)
	}
	ownerAddr := common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	if err := client.CanUnlinkOwner(ownerAddr, ts, sigBytes); err != nil {
		return fmt.Errorf("unlink request verification failed: %w", err)
	}

	prettyResp, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal linking response")
		return err
	}

	selector, err := strconv.ParseUint(resp.ChainSelector, 10, 64)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to parse chain selector")
	}
	ChainName, err := settings.GetChainNameByChainSelector(selector)
	if err != nil {
		h.log.Error().Err(err).Uint64("selector", selector).Msg("failed to get chain name")
		return err
	}

	h.log.Debug().Msg("\nRaw linking response payload:\n\n" + string(prettyResp))

	h.log.Info().Msg("")
	h.log.Info().Msg("Ownership linking initialized successfully!")
	h.log.Info().Msg("")
	h.log.Info().Msg("Next steps:")
	h.log.Info().Msg("")
	h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("      Chain:            %s", ChainName)
	h.log.Info().Msgf("      Contract Address: %s", resp.ContractAddress)
	h.log.Info().Msg("")
	h.log.Info().Msg("   2. Use the following transaction data:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("      %s", resp.TransactionData)
	h.log.Info().Msg("")

	return nil
}
