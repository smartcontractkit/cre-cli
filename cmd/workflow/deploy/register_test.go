package deploy

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/dev-platform/cmd/client"
)

func makeWorkflowID(hexNoPrefix string) [32]byte {
	var out [32]byte
	b := common.Hex2Bytes(hexNoPrefix)
	copy(out[:], b)
	return out
}

func sampleParams() client.RegisterWorkflowV2Parameters {
	return client.RegisterWorkflowV2Parameters{
		WorkflowName: "workflow2",
		Tag:          "workflow2",
		WorkflowID:   makeWorkflowID("002880c41c6825f576dd99401b3d4354558fc6fd95092b48112ee4a367daed19"),
		Status:       1, // paused
		DonFamily:    "sandbox_don_family",
		BinaryURL:    "https://example.com/binary.wasm",
		ConfigURL:    "https://example.com/config.json",
		Attributes:   []byte{},
	}
}

func TestPackUpsertTxData_MatchesABIEncoding(t *testing.T) {
	params := sampleParams()

	got, err := packUpsertTxData(params)
	if err != nil {
		t.Fatalf("packUpsertTxData() error = %v", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_v2_wrapper.WorkflowRegistryMetaData.ABI))
	if err != nil {
		t.Fatalf("failed to parse ABI: %v", err)
	}
	expectedBytes, err := contractABI.Pack(
		"upsertWorkflow",
		params.WorkflowName,
		params.Tag,
		params.WorkflowID,
		params.Status,
		params.DonFamily,
		params.BinaryURL,
		params.ConfigURL,
		params.Attributes,
		params.KeepAlive,
	)
	if err != nil {
		t.Fatalf("failed to pack expected bytes: %v", err)
	}
	want := hex.EncodeToString(expectedBytes)

	if got != want {
		t.Fatalf("hex mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestPackUpsertTxData_SelectorPrefix(t *testing.T) {
	params := sampleParams()

	txData, err := packUpsertTxData(params)
	if err != nil {
		t.Fatalf("packUpsertTxData() error = %v", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_v2_wrapper.WorkflowRegistryMetaData.ABI))
	if err != nil {
		t.Fatalf("failed to parse ABI: %v", err)
	}
	m, ok := contractABI.Methods["upsertWorkflow"]
	if !ok {
		t.Fatalf("ABI missing method upsertWorkflow")
	}
	selectorHex := hex.EncodeToString(m.ID)

	if len(txData) < len(selectorHex) || txData[:len(selectorHex)] != selectorHex {
		t.Fatalf("selector prefix mismatch: got %q, want %q", txData[:len(selectorHex)], selectorHex)
	}
}

func TestPackUpsertTxData_Deterministic(t *testing.T) {
	params := sampleParams()

	a, err := packUpsertTxData(params)
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}
	b, err := packUpsertTxData(params)
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}
	if a != b {
		t.Fatalf("non-deterministic output:\nA=%s\nB=%s", a, b)
	}
}
