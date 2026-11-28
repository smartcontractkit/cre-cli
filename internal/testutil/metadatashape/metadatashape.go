// Package metadatashape checks the workflow execution metadata that a workflow
// reads through the CRE SDK (cre.ExecutionMetadata in Go, executionMetadata in
// TypeScript) against the shape the production engine produces.
//
// The contract is about type and structure, not values: the simulator may
// return placeholder IDs, but it must never return a different set of fields,
// a different type, or a different format than production, because a workflow
// that works in simulation would then break after deploy.
package metadatashape

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"testing"
)

// Field names as the test fixtures log them (the TypeScript field names).
const (
	FieldWorkflowID    = "workflowID"
	FieldExecutionID   = "executionID"
	FieldWorkflowOwner = "workflowOwner"
	FieldDonID         = "donID"
	FieldDonF          = "donF"
)

var (
	// 32-byte lowercase hex, no 0x prefix.
	hex32 = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// 20-byte lowercase hex, no 0x prefix.
	hex20 = regexp.MustCompile(`^[0-9a-f]{40}$`)

	stringFields = map[string]*regexp.Regexp{
		FieldWorkflowID:    hex32,
		FieldExecutionID:   hex32,
		FieldWorkflowOwner: hex20,
	}
	uintFields = []string{FieldDonID, FieldDonF}
)

// Parse decodes the JSON object a fixture logged. Numbers are kept as
// json.Number so that whole numbers and fractions can be told apart.
func Parse(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode metadata JSON: %w", err)
	}
	if obj == nil {
		return nil, errors.New("metadata JSON is null, want an object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the metadata JSON object")
	}
	return obj, nil
}

// Validate checks obj against the metadata contract and returns every
// violation it finds, or nil. The rules, which must all hold:
//
//  1. The keys are exactly workflowID, executionID, workflowOwner, donID and
//     donF. None is missing and none is extra.
//  2. workflowID, executionID and workflowOwner are strings. donID and donF
//     are whole numbers in [0, 2^32-1] (uint32 in the Go SDK).
//  3. workflowID and executionID are 64 lowercase hex characters and
//     workflowOwner is 40, all without a 0x prefix. executionID is not empty:
//     a workflow can only read metadata during an execution, never during
//     trigger subscription.
func Validate(obj map[string]any) error {
	var errs []error

	allowed := map[string]bool{}
	for name := range stringFields {
		allowed[name] = true
	}
	for _, name := range uintFields {
		allowed[name] = true
	}

	for name := range allowed {
		if _, ok := obj[name]; !ok {
			errs = append(errs, fmt.Errorf("missing field %q", name))
		}
	}
	for name := range obj {
		if !allowed[name] {
			errs = append(errs, fmt.Errorf("unexpected field %q", name))
		}
	}

	for name, pattern := range stringFields {
		v, ok := obj[name]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("field %q is %T, want string", name, v))
			continue
		}
		if !pattern.MatchString(s) {
			errs = append(errs, fmt.Errorf("field %q = %q, want lowercase hex matching %s with no 0x prefix", name, s, pattern))
		}
	}

	for _, name := range uintFields {
		v, ok := obj[name]
		if !ok {
			continue
		}
		if err := checkUint32(v); err != nil {
			errs = append(errs, fmt.Errorf("field %q: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

// Require fails the test if obj does not satisfy Validate.
func Require(t testing.TB, obj map[string]any) {
	t.Helper()
	if err := Validate(obj); err != nil {
		t.Fatalf("workflow execution metadata does not match the contract:\n%v\nobject: %v", err, obj)
	}
}

func checkUint32(v any) error {
	switch n := v.(type) {
	case json.Number:
		if _, err := strconv.ParseUint(n.String(), 10, 32); err != nil {
			return fmt.Errorf("%s is not a whole number in [0, %d]", n, uint64(math.MaxUint32))
		}
		return nil
	case float64:
		if n < 0 || n > math.MaxUint32 || n != math.Trunc(n) {
			return fmt.Errorf("%v is not a whole number in [0, %d]", n, uint64(math.MaxUint32))
		}
		return nil
	default:
		return fmt.Errorf("is %T, want number", v)
	}
}
