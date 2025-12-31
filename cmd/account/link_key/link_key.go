package link_key

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	commonconfig "github.com/smartcontractkit/chainlink-common/pkg/config"
	crecontracts "github.com/smartcontractkit/chainlink/deployment/cre/contracts"
	"github.com/smartcontractkit/chainlink/deployment/cre/workflow_registry/v2/changeset"
	"github.com/smartcontractkit/mcms/types"
	"sigs.k8s.io/yaml"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
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

type ChangesetFile struct {
	Environment string      `json:"environment"`
	Domain      string      `json:"domain"`
	Changesets  []Changeset `json:"changesets"`
}

type Changeset struct {
	LinkOwner LinkOwner `json:"LinkOwner"`
}

type LinkOwner struct {
	Payload changeset.UserLinkOwnerInput `json:"payload"`
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
	settings.AddTxnTypeFlags(cmd)
	settings.AddSkipConfirmation(cmd)
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
	wrc            *client.WorkflowRegistryV2Client

	validated bool

	wg     sync.WaitGroup
	wrcErr error
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	h := handler{
		settings:       ctx.Settings,
		credentials:    ctx.Credentials,
		clientFactory:  ctx.ClientFactory,
		log:            ctx.Logger,
		environmentSet: ctx.EnvironmentSet,
		stdin:          stdin,
		wg:             sync.WaitGroup{},
		wrcErr:         nil,
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
		if err != nil {
			h.wrcErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		h.wrc = wrc
	}()

	return &h
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

	h.displayDetails()

	if in.WorkflowOwnerLabel == "" {
		if err := prompt.SimplePrompt(h.stdin, "Provide a label for your owner address", func(inputLabel string) error {
			in.WorkflowOwnerLabel = inputLabel
			return nil
		}); err != nil {
			return err
		}
	}

	h.wg.Wait()
	if h.wrcErr != nil {
		return h.wrcErr
	}

	linked, err := h.checkIfAlreadyLinked()
	if err != nil {
		return err
	}
	if linked {
		return nil
	}

	fmt.Printf("Starting linking: owner=%s, label=%s\n", in.WorkflowOwner, in.WorkflowOwnerLabel)

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

	if in.WorkflowRegistryContractAddress == resp.ContractAddress {
		fmt.Println("Contract address validation passed")
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
	req.Header.Set("Idempotency-Key", uuid.New().String())

	var container struct {
		InitiateLinking initiateLinkingResponse `json:"initiateLinking"`
	}

	if err := graphqlclient.New(h.credentials, h.environmentSet, h.log).
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

	ownerAddr := common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	if err := h.wrc.CanLinkOwner(ownerAddr, ts, proofBytes, sigBytes); err != nil {
		return fmt.Errorf("link request verification failed: %w", err)
	}
	txOut, err := h.wrc.LinkOwner(ts, proofBytes, sigBytes)
	if err != nil {
		return fmt.Errorf("LinkOwner failed: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		fmt.Println("Transaction confirmed")
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		fmt.Println("\n[OK] web3 address linked to your CRE organization successfully")
		fmt.Println("\nNote: Linking verification may take up to 60 seconds.")
		fmt.Println("\n→ You can now deploy workflows using this address")

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
		fmt.Println("Ownership linking initialized successfully!")
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("")
		fmt.Println("   1. Submit the following transaction on the target chain:")
		fmt.Printf("      Chain:            %s\n", ChainName)
		fmt.Printf("      Contract Address: %s\n", txOut.RawTx.To)
		fmt.Println("")
		fmt.Println("   2. Use the following transaction data:")
		fmt.Println("")
		fmt.Printf("      %x\n", txOut.RawTx.Data)
		fmt.Println("")

	case client.Changeset:
		chainSelector, err := settings.GetChainSelectorByChainName(h.environmentSet.WorkflowRegistryChainName)
		if err != nil {
			return fmt.Errorf("failed to get chain selector for chain %q: %w", h.environmentSet.WorkflowRegistryChainName, err)
		}
		minDelay, err := time.ParseDuration(h.settings.Workflow.CLDSettings.MCMSSettings.MinDelay)
		if err != nil {
			return fmt.Errorf("failed to parse min delay duration: %w", err)
		}
		validDuration, err := time.ParseDuration(h.settings.Workflow.CLDSettings.MCMSSettings.ValidDuration)
		if err != nil {
			return fmt.Errorf("failed to parse valid duration: %w", err)
		}
		csFile := ChangesetFile{
			Environment: h.settings.Workflow.CLDSettings.Environment,
			Domain:      h.settings.Workflow.CLDSettings.Domain,
			Changesets: []Changeset{
				{
					LinkOwner: LinkOwner{
						Payload: changeset.UserLinkOwnerInput{
							ValidityTimestamp: ts,
							Proof:             common.Bytes2Hex(proofBytes[:]),
							Signature:         common.Bytes2Hex(sigBytes),
							ChainSelector:     chainSelector,
							MCMSConfig: &crecontracts.MCMSConfig{
								MinDelay:     minDelay,
								MCMSAction:   types.TimelockActionSchedule,
								OverrideRoot: h.settings.Workflow.CLDSettings.MCMSSettings.OverrideRoot == "true",
								TimelockQualifierPerChain: map[uint64]string{
									chainSelector: h.settings.Workflow.CLDSettings.MCMSSettings.TimelockQualifier,
								},
								ValidDuration: commonconfig.MustNewDuration(validDuration),
							},
							WorkflowRegistryQualifier: h.settings.Workflow.CLDSettings.WorkflowRegistryQualifier,
						},
					},
				},
			},
		}

		yamlData, err := yaml.Marshal(&csFile)
		if err != nil {
			return fmt.Errorf("failed to marshal changeset to yaml: %w", err)
		}

		fileName := fmt.Sprintf("LinkOwner_%s_%d.yaml", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, time.Now().Unix())
		fullFilePath := filepath.Join(
			filepath.Clean(h.settings.Workflow.CLDSettings.CLDPath),
			"domains",
			h.settings.Workflow.CLDSettings.Domain,
			h.settings.Workflow.CLDSettings.Environment,
			"durable_pipelines",
			"inputs",
			fileName,
		)
		if err := os.WriteFile(fullFilePath, yamlData, 0600); err != nil {
			return fmt.Errorf("failed to write changeset yaml file: %w", err)
		}

		fmt.Println("")
		fmt.Println("Changeset YAML file generated!")
		fmt.Printf("File: %s\n", fullFilePath)
		fmt.Println("")

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}

	fmt.Println("Linked successfully")
	return nil
}

func (h *handler) checkIfAlreadyLinked() (bool, error) {
	ownerAddr := common.HexToAddress(h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	fmt.Println("\nChecking existing registrations...")

	linked, err := h.wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return false, fmt.Errorf("failed to check owner link status: %w", err)
	}

	if linked {
		fmt.Println("web3 address already linked")
		return true, nil
	}

	fmt.Println("✓ No existing link found for this address")
	return false, nil
}

func (h *handler) displayDetails() {
	fmt.Println("Linking web3 key to your CRE organization")
	fmt.Printf("Target : \t\t %s\n", h.settings.User.TargetName)
	fmt.Printf("✔ Using Address : \t %s\n\n", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
}
