package workflow

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	workflow_registry_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
)

func FormatWorkflowsTable(metadata []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView) string {
	if (len(metadata)) == 0 {
		return "No workflow available for owner"
	}

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Name", "ID", "DON", "Status"})

	for _, m := range metadata {
		t.AppendRow(table.Row{
			m.WorkflowName,
			hex.EncodeToString(m.WorkflowId[:]),
			m.DonFamily,
			getStatusString(m.Status),
		})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignLeft},
		{Number: 2, Align: text.AlignLeft},
		{Number: 3, Align: text.AlignCenter},
		{Number: 4, Align: text.AlignCenter},
	})

	t.SortBy([]table.SortBy{
		{Name: "Name", Mode: table.Asc},
	})

	return t.Render()
}

func FormatWorkflow(metadata workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView) string {
	var sb strings.Builder
	sb.WriteString("Workflow Metadata:\n")

	fmt.Fprintf(&sb, "  * Name: %s\n", metadata.WorkflowName)
	fmt.Fprintf(&sb, "  * ID: %s\n", hex.EncodeToString(metadata.WorkflowId[:]))
	fmt.Fprintf(&sb, "  * Status: %s\n", getStatusString(metadata.Status))
	fmt.Fprintf(&sb, "  * DON: %s\n", metadata.DonFamily)
	fmt.Fprintf(&sb, "  * Owner: %s\n", metadata.Owner.Hex())

	fmt.Fprintf(&sb, "  * Binary URL: %s\n", metadata.BinaryUrl)
	if metadata.ConfigUrl == "" {
		sb.WriteString("  * Config URL: (None provided)\n")
	} else {
		fmt.Fprintf(&sb, "  * Config URL: %s\n", metadata.ConfigUrl)
	}

	return sb.String()
}

func getStatusString(status uint8) string {
	if status == 1 {
		return "\u001B[1;33mPaused\u001B[1;37m"
	}
	return "\u001B[1;32mActive\u001B[1;37m"
}
