package solana_bindings_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"encoding/binary"
	"encoding/json"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/test-go/testify/require"

	"github.com/gagliardetto/anchor-go/idl"
	anchoridlcodec "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/anchorcodec"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/common"
	my_anchor_project "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project"
)

const anyChainSelector = uint64(1337)

func TestSolanaBasic(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)
	version, err := solanaClient.GetVersion(context.Background())
	require.NoError(t, err)
	fmt.Println("version", version)
	health, err := solanaClient.GetHealth(context.Background())
	require.NoError(t, err)
	fmt.Println("health", health)
	fmt.Println(my_anchor_project.ProgramID.String())
}

func TestSolanaInit(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	dataAccountAccount, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_anchor_project.ProgramID,
	)
	ix, err := my_anchor_project.NewInitializeInstruction(
		"test-data",
		dataAccountAccount,
		pk.PublicKey(),
		solana.SystemProgramID,
	)
	require.NoError(t, err)

	res, err := common.SendAndConfirm(
		context.Background(),
		solanaClient,
		[]solana.Instruction{ix},
		pk,
		rpc.CommitmentConfirmed,
		common.AddSigners(pk),
	)
	require.NoError(t, err)
	fmt.Println("res", res.Meta.LogMessages)

}

func TestSolanaGetData(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// dataAccountAccount, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	my_anchor_project.ProgramID,
	// )

	ix3, err := my_anchor_project.NewGetInputDataInstruction("test-data")
	require.NoError(t, err)
	// ix4, err := my_anchor_project.NewGetInputDataFromAccountInstruction("test-data", dataAccountAccount)
	// require.NoError(t, err)
	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3, ix4}, pk, rpc.CommitmentConfirmed)
	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3}, pk, rpc.CommitmentConfirmed)

	require.NoError(t, err)
	for _, log := range res.Meta.LogMessages {
		if strings.Contains(log, "Program log:") {
			fmt.Println("log", log)
		}
	}
}

func TestSolanaReadAccount(t *testing.T) {
	// create client
	solanaClient := rpc.New("http://localhost:8899")
	// find pda
	dataAccountAddress, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_anchor_project.ProgramID,
	)
	// call rpc
	resp, err := solanaClient.GetAccountInfoWithOpts(
		context.Background(),
		dataAccountAddress,
		&rpc.GetAccountInfoOpts{
			Commitment: rpc.CommitmentConfirmed,
		},
	)
	require.NoError(t, err, "failed to get account info")
	// parse account info
	data, err := my_anchor_project.ParseAccount_DataAccount(resp.Value.Data.GetBinary())
	require.NoError(t, err, "failed to parse account info")
	fmt.Println("data", data)

	// data2, err := my_anchor_project.ReadAccount_DataAccount([][]byte{[]byte("test")}, solanaClient)
	// require.NoError(t, err, "failed to read account info")
	// fmt.Println("data2", data2)
}

func TestSolanaWriteAccount(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	dataAccountAddress, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_anchor_project.ProgramID,
	)
	ix, err := my_anchor_project.NewUpdateDataInstruction("test-data-new", dataAccountAddress)
	require.NoError(t, err)

	// ix2, err := my_anchor_project.NewUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress)
	// require.NoError(t, err)

	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix, ix2}, pk, rpc.CommitmentConfirmed)
	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix}, pk, rpc.CommitmentConfirmed)

	require.NoError(t, err)
	fmt.Println("res", res.Meta.LogMessages)

	output, err := common.ExtractTypedReturnValue(context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String(), func(b []byte) string {
		require.Len(t, b, int(binary.LittleEndian.Uint32(b[:4]))+4) // the first 4 bytes just encodes the length
		return string(b[4:])
	})
	require.NoError(t, err)
	fmt.Println("output", output)

	// output2, err := common.ExtractAnchorTypedReturnValue[my_anchor_project.UpdateResponse](context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String())
	// require.NoError(t, err)
	// fmt.Println("output2", output2)

	// output3, err := my_anchor_project.SendUpdateDataInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("output3", output3)

	// output4, err := my_anchor_project.SendUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("output4", output4.Data)
}

func findSubkeyPaths(data any) []string {
	v := reflect.ValueOf(data)

	// unwrap pointers/interfaces to the concrete value
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	var res []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// if the field is "deep zero", skip it entirely
		if isDeepZero(fv) {
			continue
		}

		// unwrap ptr/interface for kind checks below
		w := fv
		// for w.Kind() == reflect.Pointer || w.Kind() == reflect.Interface {
		// 	w = w.Elem()
		// }

		if w.Kind() == reflect.Struct {
			for _, sub := range findSubkeyPaths(fv.Interface()) {
				res = append(res, field.Name+"."+sub)
			}
			continue
		}

		res = append(res, field.Name)
	}

	return res
}

// isDeepZero reports whether v is nil/zero, treating pointers/interfaces
// as zero if nil, and structs as zero only if ALL fields are deep-zero.
func isDeepZero(v reflect.Value) bool {
	// nil ptr/interface is zero
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			if !isDeepZero(v.Field(i)) {
				return false
			}
		}
		return true
	}

	// for everything else, rely on reflect's zero check
	return v.IsZero()
}

func TestSubKeyPaths(t *testing.T) {
	data := struct {
		Sender      string
		ValueStruct struct {
			NativeField string
			StructField struct {
				Field1 string
				Field2 string
			}
		}
		Value struct {
			NativeField string
			StructField struct {
				Field1 string
				Field2 string
			}
		}
	}{
		// Sender: "0x1234567890123456789012345678901234567890",
		Value: struct {
			NativeField string
			StructField struct {
				Field1 string
				Field2 string
			}
		}{
			// NativeField: "test-native-field",
			StructField: struct {
				Field1 string
				Field2 string
			}{
				Field1: "test-field1",
				Field2: "test-field2",
			},
		},
	}
	subkeyPaths := findSubkeyPaths(data)
	fmt.Println(subkeyPaths)
}

func TestDataWithIdl(t *testing.T) {
	data := my_anchor_project.DataAccount{
		Data: "test-data",
		Data2: my_anchor_project.DataAccount2{
			Data2: "test-data2",
		},
	}
	fmt.Println(data)
	myidl, err := idl.Parse([]byte(my_anchor_project.IDL))
	require.NoError(t, err)
	eventName := "DataAccount"
	for _, typ := range myidl.Types {
		if typ.Name != eventName {
			continue
		}
		switch vv := typ.Ty.(type) {
		case *idl.IdlTypeDefTyStruct:
			switch xxx := vv.Fields.(type) {
			case idl.IdlDefinedFieldsNamed:
				for _, field := range xxx {
					fmt.Println("field", field.Name, field.Ty)

				}
			default:
				panic(fmt.Errorf("unhandled type: %T", xxx))
			}
		default:
			panic(fmt.Errorf("unhandled type: %T", vv))
		}
	}
}

/*
anchor-go \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/my_anchor_project.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project \
  --program-id 2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN \
  --no-go-mod

./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/my_project.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project \
  --program-id 2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN \
  --no-go-mod

	 go build -ldflags "-w" -o anchor;./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/data_storage.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/data_storage \
  --program-id ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL \
  --no-go-mod

*/

var IDL = "{\"address\":\"2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN\",\"metadata\":{\"name\":\"my_anchor_project\",\"version\":\"0.1.0\",\"spec\":\"0.1.0\",\"description\":\"Created with Anchor\"},\"instructions\":[{\"name\":\"get_input_data\",\"discriminator\":[30,17,181,230,219,1,5,138],\"accounts\":[],\"args\":[{\"name\":\"input\",\"type\":\"string\"}]},{\"name\":\"initialize\",\"discriminator\":[175,175,109,31,13,152,155,237],\"accounts\":[{\"name\":\"data_account\",\"writable\":true,\"pda\":{\"seeds\":[{\"kind\":\"const\",\"value\":[116,101,115,116]}]}},{\"name\":\"user\",\"writable\":true,\"signer\":true},{\"name\":\"system_program\",\"address\":\"11111111111111111111111111111111\"}],\"args\":[{\"name\":\"input\",\"type\":\"string\"}]},{\"name\":\"log_access\",\"discriminator\":[196,55,194,24,5,224,161,204],\"accounts\":[{\"name\":\"user\",\"docs\":[\"The caller — whoever invokes the instruction.\"],\"signer\":true}],\"args\":[{\"name\":\"message\",\"type\":\"string\"}]},{\"name\":\"update_data\",\"discriminator\":[62,209,63,231,204,93,148,123],\"accounts\":[{\"name\":\"data_account\",\"writable\":true,\"pda\":{\"seeds\":[{\"kind\":\"const\",\"value\":[116,101,115,116]}]}}],\"args\":[{\"name\":\"new_data\",\"type\":\"string\"}]}],\"accounts\":[{\"name\":\"DataAccount\",\"discriminator\":[85,240,182,158,76,7,18,233]}],\"events\":[{\"name\":\"AccessLogged\",\"discriminator\":[243,53,225,71,64,120,109,25]},{\"name\":\"DataUpdated\",\"discriminator\":[110,104,69,204,253,168,30,91]}],\"errors\":[{\"code\":6000,\"name\":\"DataTooLong\",\"msg\":\"Data too long\"}],\"types\":[{\"name\":\"AccessLogged\",\"type\":{\"kind\":\"struct\",\"fields\":[{\"name\":\"caller\",\"type\":\"pubkey\"},{\"name\":\"message\",\"type\":\"string\"}]}},{\"name\":\"DataAccount\",\"type\":{\"kind\":\"struct\",\"fields\":[{\"name\":\"data\",\"type\":\"string\"},{\"name\":\"data2\",\"type\":{\"defined\":{\"name\":\"DataAccount2\"}}}]}},{\"name\":\"DataAccount2\",\"type\":{\"kind\":\"struct\",\"fields\":[{\"name\":\"data2\",\"type\":\"string\"}]}},{\"name\":\"DataUpdated\",\"type\":{\"kind\":\"struct\",\"fields\":[{\"name\":\"sender\",\"type\":\"pubkey\"},{\"name\":\"value\",\"type\":\"string\"}]}}]}"

func TestParseIDL(t *testing.T) {
	type ynIdlType2 struct {
		Types anchoridlcodec.IdlTypeDefSlice `json:"types,omitempty"`
	}

	var myIdl ynIdlType2
	err := json.Unmarshal([]byte(IDL), &myIdl)
	require.NoError(t, err)
	fmt.Println("myIdl", myIdl)

	myevent := anchoridlcodec.IdlEvent{}

	fmt.Println("myevent", myevent)
	return
}
