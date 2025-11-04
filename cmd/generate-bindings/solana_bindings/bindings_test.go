package solana_bindings_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/test-go/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/common"
	// my_anchor_project "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project"
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
	// fmt.Println(my_anchor_project.ProgramID.String())
}

func TestSolanaInit(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// dataAccountAccount, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	my_anchor_project.ProgramID,
	// )
	// ix, err := my_anchor_project.NewInitializeInstruction(
	// 	"test-data",
	// 	dataAccountAccount,
	// 	pk.PublicKey(),
	// 	solana.SystemProgramID,
	// )
	require.NoError(t, err)

	res, err := common.SendAndConfirm(
		context.Background(),
		solanaClient,
		[]solana.Instruction{},
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

	// ix3, err := my_anchor_project.NewGetInputDataInstruction("test-data")
	require.NoError(t, err)
	// ix4, err := my_anchor_project.NewGetInputDataFromAccountInstruction("test-data", dataAccountAccount)
	// require.NoError(t, err)
	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3, ix4}, pk, rpc.CommitmentConfirmed)
	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{}, pk, rpc.CommitmentConfirmed)

	require.NoError(t, err)
	for _, log := range res.Meta.LogMessages {
		if strings.Contains(log, "Program log:") {
			fmt.Println("log", log)
		}
	}
}

func TestSolanaReadAccount(t *testing.T) {
	// create client
	// solanaClient := rpc.New("http://localhost:8899")
	// // find pda
	// dataAccountAddress, _, err := solana.FindProgramAddress(
	// 	[][]byte{[]byte("test")},
	// 	// my_anchor_project.ProgramID,
	// )
	// // call rpc
	// resp, err := solanaClient.GetAccountInfoWithOpts(
	// 	context.Background(),
	// 	dataAccountAddress,
	// 	&rpc.GetAccountInfoOpts{
	// 		Commitment: rpc.CommitmentConfirmed,
	// 	},
	// )
	// require.NoError(t, err, "failed to get account info")
	// // parse account info
	// // data, err := my_anchor_project.ParseAccount_DataAccount(resp.Value.Data.GetBinary())
	// require.NoError(t, err, "failed to parse account info")
	// fmt.Println("data", data)

	// data2, err := my_anchor_project.ReadAccount_DataAccount([][]byte{[]byte("test")}, solanaClient)
	// require.NoError(t, err, "failed to read account info")
	// fmt.Println("data2", data2)
}

func TestSolanaWriteAccount(t *testing.T) {
	// solanaClient := rpc.New("http://localhost:8899")
	// pk, err := solana.NewRandomPrivateKey()
	// require.NoError(t, err)
	// common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	// // dataAccountAddress, _, err := solana.FindProgramAddress(
	// // 	[][]byte{[]byte("test")},
	// // 	// my_anchor_project.ProgramID,
	// // )
	// // ix, err := my_anchor_project.NewUpdateDataInstruction("test-data-new", dataAccountAddress)
	// require.NoError(t, err)

	// // ix2, err := my_anchor_project.NewUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress)
	// // require.NoError(t, err)

	// // res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix, ix2}, pk, rpc.CommitmentConfirmed)
	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix}, pk, rpc.CommitmentConfirmed)

	// require.NoError(t, err)
	// fmt.Println("res", res.Meta.LogMessages)

	// // output, err := common.ExtractTypedReturnValue(context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String(), func(b []byte) string {
	// // 	require.Len(t, b, int(binary.LittleEndian.Uint32(b[:4]))+4) // the first 4 bytes just encodes the length
	// // 	return string(b[4:])
	// // })
	// require.NoError(t, err)
	// fmt.Println("output", output)

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
