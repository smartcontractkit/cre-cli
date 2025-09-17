package unlink_key

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	"github.com/smartcontractkit/dev-platform/cmd/client"
	"github.com/smartcontractkit/dev-platform/internal/client/graphqlclient"
	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/credentials"
	"github.com/smartcontractkit/dev-platform/internal/environments"
	"github.com/smartcontractkit/dev-platform/internal/prompt"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
	"github.com/smartcontractkit/dev-platform/internal/settings"
	"github.com/smartcontractkit/dev-platform/internal/validation"
)

const (
	environment = "PRODUCTION_TESTNET"
)

type UnlinkActionOption struct {
	Action      string
	Description string
	ID          uint32
}

var unlinkActionOptions = []UnlinkActionOption{
	{Action: "NONE", Description: "No action prior to unlinking", ID: 1},
	{Action: "REMOVE_WORKFLOWS", Description: "Remove all workflows owned by the owner prior to unlinking", ID: 2},
	{Action: "PAUSE_WORKFLOWS", Description: "Pause all workflows owned by the owner prior to unlinking", ID: 3},
}

type Inputs struct {
	WorkflowOwner                   string `validate:"workflow_owner"`
	WorkflowOwnerType               string `validate:"omitempty"`
	ActionID                        uint32 `validate:"omitempty,gt=0,lt=4"`
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
	cmd.Flags().Uint32P("action-id", "a", 0, "ID of the unlink action option to use")
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
		WorkflowOwnerType:               h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType,
		WorkflowRegistryContractAddress: h.environmentSet.WorkflowRegistryAddress,
		ActionID:                        v.GetUint32("action-id"),
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

	action, err := h.selectAction(in)
	if err != nil {
		return err
	}

	resp, err := h.callInitiateUnlinking(context.Background(), in, action)
	if err != nil {
		return err
	}

	if in.WorkflowRegistryContractAddress == resp.ContractAddress {
		h.log.Info().Msg("Contract address validation passed")
	} else {
		return fmt.Errorf("contract address validation failed")
	}

	switch in.WorkflowOwnerType {
	case constants.WorkflowOwnerTypeMSIG:
		return h.unlinkOwnerUsingMSIG(resp)
	default:
		return h.unlinkOwnerUsingEOA(in.WorkflowOwner, resp)
	}
}

func (h *handler) selectAction(in Inputs) (string, error) {
	if in.ActionID != 0 {
		opt, err := h.getActionOptionByID(in.ActionID)
		if err != nil {
			return "", fmt.Errorf("invalid action ID %d: %w", in.ActionID, err)
		}
		return opt.Action, nil
	}
	descriptions := h.extractActionDescriptions()
	var selected UnlinkActionOption
	err := prompt.SelectPrompt(h.stdin,
		"Choose what to do with existing workflows owned by this address",
		descriptions,
		func(choice string) error {
			opt, err := h.getUnlinkActionByDescription(choice)
			if err != nil {
				return err
			}
			selected = opt
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("action selection aborted: %w", err)
	}
	return selected.Action, nil
}

func (h *handler) callInitiateUnlinking(ctx context.Context, in Inputs, action string) (initiateUnlinkingResponse, error) {
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
		"preUnlinkAction":      action,
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

	args := resp.FunctionArgs
	if len(args) < 4 {
		return fmt.Errorf("unexpected functionArgs length: %d", len(args))
	}
	actionRaw := args[3]
	actionUint, err := strconv.ParseUint(actionRaw, 10, 8)
	if err != nil {
		return fmt.Errorf("invalid action: %s", actionRaw)
	}
	actionByte := uint8(actionUint)

	client, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("client init: %w", err)
	}
	addr := common.HexToAddress(owner)
	if err := client.CanUnlinkOwner(addr, ts, sigBytes, actionByte); err != nil {
		return fmt.Errorf("unlink request verification failed: %w", err)
	}
	if err := client.UnlinkOwner(addr, ts, sigBytes, actionByte); err != nil {
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

	args := resp.FunctionArgs
	if len(args) < 4 {
		return fmt.Errorf("unexpected functionArgs length: %d", len(args))
	}
	actionRaw := args[3]
	actionUint, err := strconv.ParseUint(actionRaw, 10, 8)
	if err != nil {
		return fmt.Errorf("invalid action: %s", actionRaw)
	}
	actionByte := uint8(actionUint)

	client, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("client init: %w", err)
	}
	ownerAddr := common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	if err := client.CanUnlinkOwner(ownerAddr, ts, sigBytes, actionByte); err != nil {
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

func (h *handler) getUnlinkActionByDescription(desc string) (UnlinkActionOption, error) {
	for _, opt := range unlinkActionOptions {
		if opt.Description == desc {
			return opt, nil
		}
	}
	return UnlinkActionOption{}, errors.New("action not found")
}

func (h *handler) extractActionDescriptions() []string {
	list := make([]string, len(unlinkActionOptions))
	for i, opt := range unlinkActionOptions {
		list[i] = opt.Description
	}
	return list
}

func (h *handler) getActionOptionByID(id uint32) (UnlinkActionOption, error) {
	for _, opt := range unlinkActionOptions {
		if opt.ID == id {
			return opt, nil
		}
	}
	return UnlinkActionOption{}, fmt.Errorf("action with ID %d not found", id)
}
