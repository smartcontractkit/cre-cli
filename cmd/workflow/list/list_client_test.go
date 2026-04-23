package list_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/machinebox/graphql"

	cmdlist "github.com/smartcontractkit/cre-cli/cmd/workflow/list"
)

type listAllSeqExecutor struct {
	call int
}

func (s *listAllSeqExecutor) Execute(ctx context.Context, req *graphql.Request, resp any) error {
	s.call++
	var body []byte
	var err error
	switch s.call {
	case 1:
		data := make([]map[string]string, cmdlist.DefaultPageSize)
		for i := range data {
			data[i] = map[string]string{
				"name":           "mock-wf-page",
				"workflowId":     "a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0",
				"ownerAddress":   "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
				"status":         "ACTIVE",
				"workflowSource": "contract:77766655544433322211:0xfeedface00000000000000000000000000c0ffee",
			}
		}
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": cmdlist.DefaultPageSize + 1,
				"data":  data,
			},
		})
	case 2:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{
				"count": cmdlist.DefaultPageSize + 1,
				"data": []map[string]string{
					{
						"name":           "mock-wf-last",
						"workflowId":     "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0",
						"ownerAddress":   "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
						"status":         "ACTIVE",
						"workflowSource": "private",
					},
				},
			},
		})
	default:
		body, err = json.Marshal(map[string]any{
			"workflows": map[string]any{"count": cmdlist.DefaultPageSize + 1, "data": []any{}},
		})
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
}

func TestListAll_PaginatesAndMapsRows(t *testing.T) {
	ex := &listAllSeqExecutor{}
	got, err := cmdlist.ListAll(context.Background(), ex, cmdlist.DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != cmdlist.DefaultPageSize+1 {
		t.Fatalf("got %d workflows, want %d", len(got), cmdlist.DefaultPageSize+1)
	}
	if got[0].WorkflowSource != "contract:77766655544433322211:0xfeedface00000000000000000000000000c0ffee" {
		t.Errorf("first row source: %q", got[0].WorkflowSource)
	}
	if got[len(got)-1].Name != "mock-wf-last" || got[len(got)-1].WorkflowSource != "private" {
		t.Errorf("last row: %+v", got[len(got)-1])
	}
	if ex.call != 2 {
		t.Errorf("executor calls = %d, want 2", ex.call)
	}
}
