package link_key

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
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
	environment        = "PRODUCTION_TESTNET"
	requestProcessEOA  = "EOA"
	requestProcessMSIG = "MSIG"
)

type Inputs struct {
	// TODO: Add validation for WorkflowOwnerLabel
	WorkflowOwnerLabel              string `validate:"omitempty"`
	WorkflowOwner                   string `validate:"required,workflow_owner"`
	WorkflowRegistryContractAddress string `validate:"required"`
}

type initiateLinkingResponse struct {
	OwnershipProofHash string   `json:"ownershipProofHash"`
	WorkflowOwnerAddr  string   `json:"workflowOwnerAddress"`
	ValidUntil         string   `json:"validUntil"`
	Signature          string   `json:"signature"`
	ChainSelector      string   `json:"chainSelector"`
	ContractAddress    string   `json:"contractAddress"`
	TransactionData    string   `json:"transactionData"`
	FunctionSignature  string   `json:"functionSignature"`
	FunctionArgs       []string `json:"functionArgs"`
}

func Exec(ctx *runtime.Context, in Inputs) error {
	h := newHandler(ctx, os.Stdin)

	if err := h.ValidateInputs(in); err != nil {
		return err
	}
	return h.Execute(in)
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link-key",
		Short: "Link a public key address to your account",
		Long:  `Link a public key address to your account for workflow operations.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeContext, cmd.InOrStdin())
			inputs, err := h.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			if err := h.ValidateInputs(inputs); err != nil {
				return err
			}

			return h.Execute(inputs)
		},
	}
	settings.AddRawTxFlag(cmd)
	cmd.Flags().StringP("owner-label", "l", "", "Label for the workflow owner")

	return cmd
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
		WorkflowOwnerLabel:              v.GetString("owner-label"),
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

	if in.WorkflowOwnerLabel == "" {
		if err := prompt.SimplePrompt(h.stdin, "Provide a label for your owner address", func(inputLabel string) error {
			in.WorkflowOwnerLabel = inputLabel
			return nil
		}); err != nil {
			return err
		}
	}

	h.log.Info().
		Str("owner", in.WorkflowOwner).
		Str("label", in.WorkflowOwnerLabel).
		Msg("Starting linking")

	resp, err := h.callInitiateLinking(context.Background(), in)
	if err != nil {
		return err
	}

	prettyResp, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal linking response")
		return err
	}

	h.log.Debug().Msg("\nRaw linking response payload:\n\n" + string(prettyResp))

	fileName := fmt.Sprintf("linking_%s_%d.json", resp.WorkflowOwnerAddr, time.Now().Unix())
	if err := os.WriteFile(fileName, prettyResp, 0600); err != nil {
		h.log.Error().Err(err).Msg("failed to write linking response to file")
		return err
	}

	if in.WorkflowRegistryContractAddress == resp.ContractAddress {
		h.log.Info().Msg("Contract address validation passed")
	} else {
		h.log.Warn().Msg("The workflowRegistryContractAddress in your settings does not match the one returned by the server")
		return fmt.Errorf("contract address validation failed")
	}

	if err := h.linkOwner(resp); err != nil {
		return fmt.Errorf("linking failed: %w", err)
	}

	return nil
}

func (h *handler) callInitiateLinking(ctx context.Context, in Inputs) (initiateLinkingResponse, error) {
	const mutation = `
mutation InitiateLinking($request: InitiateLinkingRequest!) {
  initiateLinking(request: $request) {
    ownershipProofHash
    workflowOwnerAddress
    validUntil
    signature
    chainSelector
    contractAddress
    transactionData
    functionSignature
    functionArgs
  }
}`

	var requestProcess string
	switch h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType {
	case constants.WorkflowOwnerTypeMSIG:
		requestProcess = requestProcessMSIG
	case constants.WorkflowOwnerTypeEOA:
		requestProcess = requestProcessEOA
	default:
		return initiateLinkingResponse{}, fmt.Errorf("invalid workflow owner type: %s", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType)
	}

	req := graphql.NewRequest(mutation)
	reqVariables := map[string]any{
		"workflowOwnerAddress": in.WorkflowOwner,
		"workflowOwnerLabel":   in.WorkflowOwnerLabel,
		"environment":          environment,
		"requestProcess":       requestProcess,
	}
	req.Var("request", reqVariables)

	var container struct {
		InitiateLinking initiateLinkingResponse `json:"initiateLinking"`
	}

	if err := graphqlclient.New(h.credentials, h.environmentSet).
		Execute(ctx, req, &container); err != nil {
		return initiateLinkingResponse{}, fmt.Errorf("graphql request failed: %w", err)
	}

	h.log.Debug().Interface("response", container).
		Msg("Received GraphQL response from initiate linking")

	return container.InitiateLinking, nil
}

func (h *handler) linkOwner(resp initiateLinkingResponse) error {
	expiresAt, err := time.Parse(time.RFC3339, resp.ValidUntil)
	if err != nil {
		return fmt.Errorf("invalid validUntil format: %w", err)
	}
	if time.Now().UTC().After(expiresAt) {
		return fmt.Errorf("the request has expired")
	}

	ts := big.NewInt(expiresAt.Unix())

	var proofBytes [32]byte
	decoded, err := hex.DecodeString(strings.TrimPrefix(resp.OwnershipProofHash, "0x"))
	if err != nil {
		return fmt.Errorf("error decoding proof")
	}
	if len(decoded) != 32 {
		return fmt.Errorf("proof hash must be 32 bytes, got %d", len(decoded))
	}
	copy(proofBytes[:], decoded)

	sigBytes, err := hex.DecodeString(strings.TrimPrefix(resp.Signature, "0x"))
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("wrc init: %w", err)
	}

	ownerAddr := common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	if err := wrc.CanLinkOwner(ownerAddr, ts, proofBytes, sigBytes); err != nil {
		return fmt.Errorf("link request verification failed: %w", err)
	}
	txOut, err := wrc.LinkOwner(ts, proofBytes, sigBytes)
	if err != nil {
		return fmt.Errorf("LinkOwner failed: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		h.log.Info().Msgf("Transaction submitted: %s", txOut.Hash)

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

		h.log.Info().Msg("")
		h.log.Info().Msg("Ownership linking initialized successfully!")
		h.log.Info().Msg("")
		h.log.Info().Msg("Next steps:")
		h.log.Info().Msg("")
		h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
		h.log.Info().Msgf("      Chain:            %s", ChainName)
		h.log.Info().Msgf("      Contract Address: %s", txOut.RawTx.To)
		h.log.Info().Msg("")
		h.log.Info().Msg("   2. Use the following transaction data:")
		h.log.Info().Msg("")
		h.log.Info().Msgf("      %x", txOut.RawTx.Data)
		h.log.Info().Msg("")
	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}

	h.log.Info().Msg("Linked successfully")
	return nil
}
