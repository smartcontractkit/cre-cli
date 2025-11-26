package bindings

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/smartcontractkit/chainlink-common/pkg/types/query/primitives"
	solana "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/capabilities/blockchain/solana"
	realSolana "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
)

// No-pointers, strict type check.
func ValidateSubKeyPathAndValue[T any](inputs []solana.SubKeyPathAndFilter) ([][]string, []solana.SubkeyFilterCriteria, error) {
	var zero T
	root := reflect.TypeOf(zero)
	if root.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("T must be a struct, got %v", root.Kind())
	}

	paths := make([][]string, 0, len(inputs))
	filters := make([]solana.SubkeyFilterCriteria, 0, len(inputs))

	for i, in := range inputs {
		parts := strings.Split(in.SubkeyPath, ".")
		if len(parts) == 0 {
			return nil, nil, fmt.Errorf("empty subkey path at index %d", i)
		}

		leafT, err := resolveLeafTypeNoPtr(root, parts)
		if err != nil {
			return nil, nil, fmt.Errorf("path %q: %w", in.SubkeyPath, err)
		}
		if leafT.Kind() == reflect.Struct {
			return nil, nil, fmt.Errorf("path %q resolves to a struct (%s); expected a scalar/leaf", in.SubkeyPath, leafT)
		}

		// Strict: require exact/assignable dynamic type (no conversions).
		if in.Value == nil {
			return nil, nil, fmt.Errorf("path %q: got <nil> for non-pointer type %s", in.SubkeyPath, leafT)
		}
		valT := reflect.TypeOf(in.Value)
		if !valT.AssignableTo(leafT) {
			return nil, nil, fmt.Errorf("path %q: value type %s not assignable to field type %s", in.SubkeyPath, valT, leafT)
		}

		paths = append(paths, parts)
		filters = append(filters, solana.SubkeyFilterCriteria{
			SubkeyIndex: uint64(i),
			Comparers:   []primitives.ValueComparator{{Value: in.Value, Operator: primitives.Eq}},
		})
	}

	return paths, filters, nil
}

func resolveLeafTypeNoPtr(root reflect.Type, parts []string) (reflect.Type, error) {
	t := root
	for idx, name := range parts {
		if t.Kind() != reflect.Struct {
			return nil, fmt.Errorf("segment %q at #%d: %s is not a struct", name, idx, t)
		}
		sf, ok := t.FieldByName(name)
		if !ok {
			return nil, fmt.Errorf("field %q not found on %s", name, t)
		}
		if sf.PkgPath != "" { // unexported
			return nil, fmt.Errorf("field %q on %s is unexported", name, t)
		}
		t = sf.Type
	}
	return t, nil
}

func ExtractEventIDL(eventName string, contractIdl *idl.Idl) (idl.IdlTypeDef, error) {
	for _, typeDef := range contractIdl.Types {
		if typeDef.Name == eventName {
			return typeDef, nil
		}
	}
	return idl.IdlTypeDef{}, fmt.Errorf("type %s not found", eventName)
}

type DecodedLog[T any] struct {
	Log  *solana.Log
	Data T
}

// this should be the same encoding expected by the solana forwarder report
func EncodeAccountList(remainingAccounts []*realSolana.AccountMeta) ([32]byte, error) {
	return [32]byte{}, nil
}
