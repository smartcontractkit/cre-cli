package deploy

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
)

type ChangesetFile struct {
	Changesets []Changeset `yaml:"changesets"`
}

type Changeset struct {
	WorkflowUpsert WorkflowUpsert `yaml:"workflow_upsert"`
}

type WorkflowUpsert struct {
	Payload Payload `yaml:"payload"`
}

type Payload struct {
	WorkflowID     string `yaml:"workflowID"`
	WorkflowName   string `yaml:"workflowName"`
	WorkflowTag    string `yaml:"workflowTag"`
	WorkflowStatus uint8  `yaml:"workflowStatus"`
	DonFamily      string `yaml:"donFamily"`
	BinaryURL      string `yaml:"binaryURL"`
	ConfigURL      string `yaml:"configURL"`
	Attributes     string `yaml:"attributes"`
	KeepAlive      bool   `yaml:"keepAlive"`
}

func (h *handler) upsert() error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	params, err := h.prepareUpsertParams()
	if err != nil {
		return err
	}
	return h.submitWorkflow(params)
}

func (h *handler) submitWorkflow(params client.RegisterWorkflowV2Parameters) error {
	return h.handleUpsert(params)
}

func (h *handler) prepareUpsertParams() (client.RegisterWorkflowV2Parameters, error) {
	workflowName := h.inputs.WorkflowName
	workflowTag := h.inputs.WorkflowTag
	binaryURL := h.inputs.BinaryURL
	configURL := h.inputs.ResolveConfigURL("")
	workflowID := h.workflowArtifact.WorkflowID

	fmt.Printf("Preparing transaction for workflowID: %s\n", workflowID)
	return client.RegisterWorkflowV2Parameters{
		WorkflowName: workflowName,
		Tag:          workflowTag,
		WorkflowID:   [32]byte(common.Hex2Bytes(workflowID)),
		Status:       0, // active
		DonFamily:    h.inputs.DonFamily,
		BinaryURL:    binaryURL,
		ConfigURL:    configURL,
		Attributes:   []byte{}, // optional
		KeepAlive:    h.inputs.KeepAlive,
	}, nil
}

func (h *handler) handleUpsert(params client.RegisterWorkflowV2Parameters) error {
	workflowName := h.inputs.WorkflowName
	workflowTag := h.inputs.WorkflowTag
	h.log.Debug().Interface("Workflow parameters", params).Msg("Registering workflow...")
	txOut, err := h.wrc.UpsertWorkflow(params)
	if err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	switch txOut.Type {
	case client.Regular:
		fmt.Println("Transaction confirmed")
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		fmt.Println("\n[OK] Workflow deployed successfully")
		fmt.Println("\nDetails:")
		fmt.Printf("   Contract address:\t%s\n", h.environmentSet.WorkflowRegistryAddress)
		fmt.Printf("   Transaction hash:\t%s\n", txOut.Hash)
		fmt.Printf("   Workflow Name:\t%s\n", workflowName)
		fmt.Printf("   Workflow ID:\t%s\n", h.workflowArtifact.WorkflowID)
		fmt.Printf("   Binary URL:\t%s\n", h.inputs.BinaryURL)
		if h.inputs.ConfigURL != nil && *h.inputs.ConfigURL != "" {
			fmt.Printf("   Config URL:\t%s\n", *h.inputs.ConfigURL)
		}

	case client.Raw:
		fmt.Println("")
		fmt.Println("MSIG workflow deployment transaction prepared!")
		fmt.Printf("To Deploy %s:%s with workflow ID: %s\n", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:]))
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Println("")
		fmt.Println("   1. Submit the following transaction on the target chain:")
		fmt.Printf("      Chain:   %s\n", h.inputs.WorkflowRegistryContractChainName)
		fmt.Printf("      Contract Address: %s\n", txOut.RawTx.To)
		fmt.Println("")
		fmt.Println("   2. Use the following transaction data:")
		fmt.Println("")
		fmt.Printf("      %x\n", txOut.RawTx.Data)
		fmt.Println("")

	case client.Changeset:
		csFile := ChangesetFile{
			Changesets: []Changeset{
				{
					WorkflowUpsert: WorkflowUpsert{
						Payload: Payload{
							WorkflowID:     hex.EncodeToString(params.WorkflowID[:]),
							WorkflowName:   params.WorkflowName,
							WorkflowTag:    params.Tag,
							WorkflowStatus: params.Status,
							DonFamily:      params.DonFamily,
							BinaryURL:      params.BinaryURL,
							ConfigURL:      params.ConfigURL,
							Attributes:     string(params.Attributes),
							KeepAlive:      params.KeepAlive,
						},
					},
				},
			},
		}

		yamlData, err := yaml.Marshal(&csFile)
		if err != nil {
			return fmt.Errorf("failed to marshal changeset to yaml: %w", err)
		}

		fileName := fmt.Sprintf("UpsertWorkflow_%s_%s.yaml", workflowName, h.workflowArtifact.WorkflowID)
		workingDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		if err := os.WriteFile(fileName, yamlData, 0600); err != nil {
			return fmt.Errorf("failed to write changeset yaml file: %w", err)
		}

		fmt.Println("")
		fmt.Println("Changeset YAML file generated!")
		fmt.Printf("File: %s\n", filepath.Join(workingDir, fileName))
		fmt.Println("")

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
}
