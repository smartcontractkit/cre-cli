package http

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/gateway"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

const (
	defaultInput   = "{}"
	requestTimeout = 90 * time.Second
)

// Inputs holds resolved and validated flag values for workflow trigger http.
type Inputs struct {
	Input        json.RawMessage
	WorkflowID   string
	OutputFormat string
}

func resolveInputs(inputFlag, workflowIDFlag, outputFormat string, jsonFlag bool, invocationDir string) (Inputs, error) {
	format, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}

	input, err := parseInput(inputFlag, invocationDir)
	if err != nil {
		return Inputs{}, err
	}

	var workflowID string
	if strings.TrimSpace(workflowIDFlag) != "" {
		if workflowID, err = normalizeWorkflowID(workflowIDFlag); err != nil {
			return Inputs{}, fmt.Errorf("--workflow-id: %w", err)
		}
	}

	return Inputs{Input: input, WorkflowID: workflowID, OutputFormat: format}, nil
}

// Handler signs a `workflows.execute` request with the wallet key and posts it to the Gateway.
type Handler struct {
	runtimeContext *runtime.Context
	out            io.Writer
}

// NewHandler builds a Handler that writes machine-readable output to out.
func NewHandler(ctx *runtime.Context, out io.Writer) *Handler {
	return &Handler{runtimeContext: ctx, out: out}
}

// Execute fires the trigger and reports the Gateway outcome. It returns an error for any
// failure, after printing the JSON result when --json is set.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	rt := h.runtimeContext
	if rt.Settings == nil {
		return fmt.Errorf("workflow settings not loaded; ensure workflow.yaml is valid")
	}

	key, err := h.signingKey()
	if err != nil {
		return err
	}

	gatewayURL := gateway.ResolveVaultGatewayURL(rt.TenantContext, rt.EnvironmentSet)
	if err := gateway.ValidateGatewayURL(gatewayURL); err != nil {
		return err
	}

	workflowID := inputs.WorkflowID
	if workflowID == "" {
		if workflowID, err = h.resolveWorkflowID(ctx); err != nil {
			return err
		}
	}
	rt.Workflow.ID = strings.TrimPrefix(workflowID, "0x")

	signed, err := newSignedRequest(key, workflowID, inputs.Input)
	if err != nil {
		return err
	}
	rt.Logger.Debug().Str("workflowID", workflowID).Str("requestID", signed.ID).Str("signer", signed.Signer.Hex()).Msg("Sending HTTP trigger request")

	// JWTs are single-use, so a retried POST would be rejected as a replay.
	gw := &gateway.HTTPClient{URL: gatewayURL, Client: &nethttp.Client{Timeout: requestTimeout}, RetryAttempts: 1}

	spinner := ui.NewSpinner()
	spinner.Start("Sending trigger request to the Gateway...")
	respBody, _, postErr := gw.PostWithBearer(signed.Body, signed.Token)
	spinner.Stop()

	result, rpcErr := parseGatewayResponse(respBody, postErr)
	out := newOutput(workflowID, signed.ID, gatewayURL, result, rpcErr)

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		if err := printJSON(h.out, out); err != nil {
			return err
		}
	} else if rpcErr == nil {
		printSummary(out)
	}

	if rpcErr != nil {
		return fmt.Errorf("trigger request %s failed: %s", signed.ID, rpcErr.Message)
	}
	return nil
}

func (h *Handler) signingKey() (*ecdsa.PrivateKey, error) {
	resolved, err := settings.ResolveEthPrivateKeyFromEnv(h.runtimeContext.Settings.User.PrivateKey(settings.EVM))
	if err != nil {
		return nil, err
	}
	if !resolved.IsSet() {
		return nil, fmt.Errorf("%s is not set; it must be one of the workflow's HTTP-trigger authorized keys", settings.EthPrivateKeyEnvVar)
	}

	key, err := crypto.HexToECDSA(resolved.Hex())
	if err != nil {
		return nil, fmt.Errorf("failed to decode the provided private key: %w", err)
	}
	return key, nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var inputFlag, workflowIDFlag, outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:    "http <workflow-folder-path>",
		Short:  "Fires a deployed workflow's HTTP trigger through the CRE Gateway",
		Hidden: true, // Kept hidden so the public CLI surface is unchanged until this is productized.
		Long: `Signs a workflows.execute JSON-RPC request with the wallet key from ` +
			`CRE_ETH_PRIVATE_KEY and posts it to the environment's Gateway. The key must be one of ` +
			`the workflow's HTTP-trigger authorized keys. The deployed workflow ID is resolved from ` +
			`the workflow name configured for the selected --target unless --workflow-id is given.`,
		Example: `cre workflow trigger http ./my-workflow --target staging
  cre workflow trigger http ./my-workflow --target staging --input '{"orderId": 42}'
  cre workflow trigger http ./my-workflow --target staging --input @payload.json --json
  cre workflow trigger http ./my-workflow --target staging --workflow-id 0x<64 hex chars>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(inputFlag, workflowIDFlag, outputFormat, jsonFlag, runtimeContext.InvocationDir)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext, cmd.OutOrStdout()).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&inputFlag, "input", defaultInput, "Trigger input as a JSON object or array, or @<path> to read it from a file")
	cmd.Flags().StringVar(&workflowIDFlag, "workflow-id", "", "Deployed workflow ID (0x...) to trigger instead of resolving it from the workflow folder")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints the result as JSON to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}
