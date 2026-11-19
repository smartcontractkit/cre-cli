package metadatashape

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	validID    = "1111111111111111111111111111111111111111111111111111111111111111"
	validExec  = "9f38bdece8892b3452c401a7a2d5a170f078ab9b2c98c591e24028ce94b33ed2"
	validOwner = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// object builds the valid JSON object and applies a textual override, so each
// case below changes exactly one thing.
func object(replacements ...[2]string) string {
	s := `{"workflowID":"` + validID + `","executionID":"` + validExec + `","workflowOwner":"` + validOwner + `","donID":1,"donF":0}`
	for _, r := range replacements {
		if !strings.Contains(s, r[0]) {
			panic("test bug: " + r[0] + " not found in base object")
		}
		s = strings.Replace(s, r[0], r[1], 1)
	}
	return s
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string // empty means valid
	}{
		{name: "valid placeholders", json: object()},
		{name: "valid zero DON values", json: object([2]string{`"donID":1`, `"donID":0`})},
		{name: "valid max uint32 DON values", json: object([2]string{`"donID":1`, `"donID":4294967295`}, [2]string{`"donF":0`, `"donF":4294967295`})},

		{name: "uppercase workflow ID", json: object([2]string{validID, strings.Repeat("A", 64)}), wantErr: `"workflowID"`},
		{name: "0x prefix on workflow ID", json: object([2]string{validID, "0x" + validID[:62]}), wantErr: `"workflowID"`},
		{name: "63 char workflow ID", json: object([2]string{validID, validID[:63]}), wantErr: `"workflowID"`},
		{name: "65 char workflow ID", json: object([2]string{validID, validID + "1"}), wantErr: `"workflowID"`},
		{name: "owner with workflow ID length", json: object([2]string{validOwner, validID}), wantErr: `"workflowOwner"`},
		{name: "owner with 0x prefix", json: object([2]string{validOwner, "0x" + validOwner[:38]}), wantErr: `"workflowOwner"`},
		{name: "non hex character", json: object([2]string{validID, "g" + validID[1:]}), wantErr: `"workflowID"`},
		{name: "empty execution ID", json: object([2]string{validExec, ""}), wantErr: `"executionID"`},
		{name: "uppercase execution ID", json: object([2]string{validExec, strings.ToUpper(validExec)}), wantErr: `"executionID"`},

		{name: "DON ID as string", json: object([2]string{`"donID":1`, `"donID":"1"`}), wantErr: `"donID"`},
		{name: "DON ID fraction", json: object([2]string{`"donID":1`, `"donID":1.5`}), wantErr: `"donID"`},
		{name: "DON ID above uint32", json: object([2]string{`"donID":1`, `"donID":4294967296`}), wantErr: `"donID"`},
		{name: "DON F negative", json: object([2]string{`"donF":0`, `"donF":-1`}), wantErr: `"donF"`},
		{name: "DON F null", json: object([2]string{`"donF":0`, `"donF":null`}), wantErr: `"donF"`},
		{name: "workflow ID as number", json: object([2]string{`"` + validID + `"`, `1`}), wantErr: `"workflowID"`},

		{name: "extra field", json: object([2]string{`"donF":0`, `"donF":0,"workflowName":"x"`}), wantErr: `unexpected field "workflowName"`},
		{name: "missing DON F", json: object([2]string{`,"donF":0`, ``}), wantErr: `missing field "donF"`},
		{name: "missing workflow ID", json: object([2]string{`"workflowID":"` + validID + `",`, ``}), wantErr: `missing field "workflowID"`},
		{name: "empty object", json: `{}`, wantErr: `missing field`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := Parse([]byte(tc.json))
			require.NoError(t, err)

			err = Validate(obj)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestValidate_ReportsEveryViolation(t *testing.T) {
	obj, err := Parse([]byte(`{"workflowID":"X","executionID":"","workflowOwner":"Y","donID":"1","donF":-1}`))
	require.NoError(t, err)

	err = Validate(obj)
	require.Error(t, err)
	for _, field := range []string{`"workflowID"`, `"executionID"`, `"workflowOwner"`, `"donID"`, `"donF"`} {
		require.Contains(t, err.Error(), field)
	}
}

func TestValidate_AcceptsFloat64Numbers(t *testing.T) {
	obj := map[string]any{
		FieldWorkflowID:    validID,
		FieldExecutionID:   validExec,
		FieldWorkflowOwner: validOwner,
		FieldDonID:         float64(1),
		FieldDonF:          float64(0),
	}
	require.NoError(t, Validate(obj))

	obj[FieldDonID] = 1.5
	require.Error(t, Validate(obj))
}

func TestParse(t *testing.T) {
	_, err := Parse([]byte(`{"a":1} trailing`))
	require.Error(t, err)

	_, err = Parse([]byte(`null`))
	require.Error(t, err)

	_, err = Parse([]byte(`[1]`))
	require.Error(t, err)

	_, err = Parse([]byte(``))
	require.Error(t, err)

	obj, err := Parse([]byte(`{"a":1}`))
	require.NoError(t, err)
	require.Len(t, obj, 1)
}
