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
	my_project "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_project"
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
	fmt.Println(my_project.ProgramID.String())
}

func TestSolanaInit(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	dataAccountAccount, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_project.ProgramID,
	)
	ix, err := my_project.NewInitializeInstruction("test-data", dataAccountAccount, pk.PublicKey(), solana.SystemProgramID)
	require.NoError(t, err)

	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix}, pk, rpc.CommitmentConfirmed)
	require.NoError(t, err)
	fmt.Println("res", res.Meta.LogMessages)

	// ix2, err := my_project.NewGetDataInstruction()
	// require.NoError(t, err)
	// res2, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix2}, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("res2", res2.Meta.LogMessages)

	// ix3, err := my_project.NewGetInputDataInstruction("test")
	// require.NoError(t, err)
	// res3, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3}, pk, rpc.CommitmentConfirmed)
	// require.NoError(t, err)
	// fmt.Println("res3", res3.Meta.LogMessages)
}

func TestSolanaGetData(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	pk, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

	dataAccountAccount, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_project.ProgramID,
	)

	ix3, err := my_project.NewGetInputDataInstruction("test-data")
	require.NoError(t, err)
	ix4, err := my_project.NewGetInputDataFromAccountInstruction("test-data", dataAccountAccount)
	require.NoError(t, err)
	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3, ix4}, pk, rpc.CommitmentConfirmed)
	require.NoError(t, err)
	for _, log := range res.Meta.LogMessages {
		if strings.Contains(log, "Program log:") {
			fmt.Println("log", log)
		}
	}
}

func TestSolanaReadAccount(t *testing.T) {
	solanaClient := rpc.New("http://localhost:8899")
	dataAccountAccount, _, err := solana.FindProgramAddress(
		[][]byte{[]byte("test")},
		my_project.ProgramID,
	)
	// var dataAccount my_project.DataAccount
	// err = common.GetAccountDataBorshInto(context.Background(), solanaClient, dataAccountAccount, rpc.CommitmentConfirmed, &dataAccount)
	// require.NoError(t, err, "failed to get account info")
	// fmt.Println("dataAccount", dataAccount)

	resp, err := solanaClient.GetAccountInfoWithOpts(
		context.Background(),
		dataAccountAccount,
		&rpc.GetAccountInfoOpts{
			Commitment: rpc.CommitmentConfirmed,
			DataSlice:  nil,
		},
	)
	require.NoError(t, err, "failed to get account info")

	data, err := my_project.ParseAccount_DataAccount(resp.Value.Data.GetBinary())
	require.NoError(t, err, "failed to parse account info")
	fmt.Println("data", data)
}
