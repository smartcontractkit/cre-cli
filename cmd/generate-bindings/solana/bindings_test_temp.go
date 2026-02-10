package solana

// "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/forwarder"
// "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/testdata/receiver"

const anyChainSelector = uint64(1337)

/*

// deploy
solana-test-validator -r \
  --upgradeable-program 5PdwLUj8VqLpA8RKGAUaReEies7kEebeQcZzcrB2R7ya /Users/yashvardhan/cre-client-program/my-project/target/deploy/forwarder.so Av3xZHYnFoW7wW4FEApAtHf8JeYauwaNm5cVLqk6MLfk \
  --upgradeable-program G5t6jDm3pmQFwW4y9KQn1iDkZrSEvC78H8cL7XaoTA3Q /Users/yashvardhan/cre-client-program/my-project/target/deploy/receiver.so Av3xZHYnFoW7wW4FEApAtHf8JeYauwaNm5cVLqk6MLfk


// update bindings
./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/forwarder.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/forwarder \
  --program-id 5PdwLUj8VqLpA8RKGAUaReEies7kEebeQcZzcrB2R7ya \
  --no-go-mod

  ./anchor \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/receiver.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/receiver \
  --program-id G5t6jDm3pmQFwW4y9KQn1iDkZrSEvC78H8cL7XaoTA3Q \
  --no-go-mod

*/

// func TestSolanaBasic(t *testing.T) {
// 	solanaClient := rpc.New("http://localhost:8899")
// 	pk, err := solana.NewRandomPrivateKey()
// 	require.NoError(t, err)
// 	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)
// 	// version, err := solanaClient.GetVersion(context.Background())
// 	// require.NoError(t, err)
// 	// fmt.Println("version", version)
// 	// health, err := solanaClient.GetHealth(context.Background())
// 	// require.NoError(t, err)
// 	// fmt.Println("health", health)
// 	// fmt.Println(forwarder.ProgramID.String())
// 	// fmt.Println(receiver.ProgramID.String())
// 	counterAccount, _, _ := solana.FindProgramAddress(
// 		[][]byte{[]byte("counter")},
// 		forwarder.ProgramID,
// 	)
// 	ix1, err := forwarder.NewReportInstruction(123456, receiver.Instruction_OnReport, counterAccount, receiver.ProgramID, solana.SystemProgramID)
// 	require.NoError(t, err)
// 	fmt.Println("ix11", ix1)
// 	// ix2, err := receiver.NewOnReportInstruction(123456)
// 	// require.NoError(t, err)
// 	// fmt.Println("ix2", ix2)
// 	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix1}, pk, rpc.CommitmentConfirmed)
// 	require.NoError(t, err)
// 	fmt.Println("res", res.Meta.LogMessages)
// }

// func TestSolanaInit(t *testing.T) {
// 	solanaClient := rpc.New("http://localhost:8899")
// 	pk, err := solana.NewRandomPrivateKey()
// 	require.NoError(t, err)
// 	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

// 	counterAccount, _, _ := solana.FindProgramAddress(
// 		[][]byte{[]byte("counter")},
// 		forwarder.ProgramID,
// 	)
// 	ix1, err := forwarder.NewInitializeInstruction(123456, pk.PublicKey(), counterAccount, solana.SystemProgramID)
// 	require.NoError(t, err)
// 	fmt.Println("ix1", ix1)
// 	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix1}, pk, rpc.CommitmentConfirmed)
// 	require.NoError(t, err)
// 	fmt.Println("res", res.Meta.LogMessages)
// }

// func TestSolanaReadAccount(t *testing.T) {
// 	// create client
// 	solanaClient := rpc.New("http://localhost:8899")
// 	// find pda
// 	counterAccountAddress, _, _ := solana.FindProgramAddress(
// 		[][]byte{[]byte("counter")},
// 		forwarder.ProgramID,
// 	)
// 	resp, err := solanaClient.GetAccountInfoWithOpts(
// 		context.Background(),
// 		counterAccountAddress,
// 		&rpc.GetAccountInfoOpts{
// 			Commitment: rpc.CommitmentConfirmed,
// 			DataSlice:  nil,
// 		},
// 	)
// 	require.NoError(t, err)
// 	counter, err := forwarder.ParseAccount_Counter(resp.Value.Data.GetBinary())
// 	require.NoError(t, err)
// 	fmt.Println("counter ", counter.Counter)
// }

// func TestSolanaInit2(t *testing.T) {
// 	solanaClient := rpc.New("http://localhost:8899")
// 	pk, err := solana.NewRandomPrivateKey()
// 	require.NoError(t, err)
// 	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)
// 	txId := uint64(3)
// 	txIdLE := common.Uint64ToLE(txId)
// 	executionStateAccount, _, _ := solana.FindProgramAddress(
// 		[][]byte{[]byte("execution_state"), txIdLE},
// 		forwarder.ProgramID,
// 	)
// 	ix1, err := forwarder.NewReport2Instruction(
// 		123456,
// 		txId,
// 		receiver.Instruction_OnReport,
// 		pk.PublicKey(),
// 		executionStateAccount,
// 		receiver.ProgramID,
// 		solana.SystemProgramID,
// 	)
// 	require.NoError(t, err)
// 	fmt.Println("ix1", ix1)
// 	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix1}, pk, rpc.CommitmentConfirmed)
// 	// require.NoError(t, err)
// 	fmt.Println("res error", err)
// 	fmt.Println("res", res)
// 	// fmt.Println("res", res.Meta.LogMessages)

// 	resp, err := solanaClient.GetAccountInfoWithOpts(
// 		context.Background(),
// 		executionStateAccount,
// 		&rpc.GetAccountInfoOpts{
// 			Commitment: rpc.CommitmentConfirmed,
// 			DataSlice:  nil,
// 		},
// 	)
// 	require.NoError(t, err)
// 	executionState, err := forwarder.ParseAccount_ExecutionState(resp.Value.Data.GetBinary())
// 	require.NoError(t, err)
// 	fmt.Println("executionState ", executionState.Success)
// 	fmt.Println("executionState ", executionState.Failure)
// 	fmt.Println("executionState ", executionState.TransmissionId)
// }

// func TestSolanaInit(t *testing.T) {
// 	solanaClient := rpc.New("http://localhost:8899")
// 	pk, err := solana.NewRandomPrivateKey()
// 	require.NoError(t, err)
// 	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

// 	// dataAccountAccount, _, err := solana.FindProgramAddress(
// 	// 	[][]byte{[]byte("test")},
// 	// 	my_anchor_project.ProgramID,
// 	// )
// 	// ix, err := my_anchor_project.NewInitializeInstruction(
// 	// 	"test-data",
// 	// 	dataAccountAccount,
// 	// 	pk.PublicKey(),
// 	// 	solana.SystemProgramID,
// 	// )
// 	require.NoError(t, err)

// 	res, err := common.SendAndConfirm(
// 		context.Background(),
// 		solanaClient,
// 		[]solana.Instruction{},
// 		pk,
// 		rpc.CommitmentConfirmed,
// 		common.AddSigners(pk),
// 	)
// 	require.NoError(t, err)
// 	fmt.Println("res", res.Meta.LogMessages)

// }

// func TestSolanaGetData(t *testing.T) {
// 	solanaClient := rpc.New("http://localhost:8899")
// 	pk, err := solana.NewRandomPrivateKey()
// 	require.NoError(t, err)
// 	common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

// 	// dataAccountAccount, _, err := solana.FindProgramAddress(
// 	// 	[][]byte{[]byte("test")},
// 	// 	my_anchor_project.ProgramID,
// 	// )

// 	// ix3, err := my_anchor_project.NewGetInputDataInstruction("test-data")
// 	require.NoError(t, err)
// 	// ix4, err := my_anchor_project.NewGetInputDataFromAccountInstruction("test-data", dataAccountAccount)
// 	// require.NoError(t, err)
// 	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix3, ix4}, pk, rpc.CommitmentConfirmed)
// 	res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{}, pk, rpc.CommitmentConfirmed)

// 	require.NoError(t, err)
// 	for _, log := range res.Meta.LogMessages {
// 		if strings.Contains(log, "Program log:") {
// 			fmt.Println("log", log)
// 		}
// 	}
// }

// func TestSolanaWriteAccount(t *testing.T) {
// 	// solanaClient := rpc.New("http://localhost:8899")
// 	// pk, err := solana.NewRandomPrivateKey()
// 	// require.NoError(t, err)
// 	// common.FundAccounts(context.Background(), []solana.PrivateKey{pk}, solanaClient, t)

// 	// // dataAccountAddress, _, err := solana.FindProgramAddress(
// 	// // 	[][]byte{[]byte("test")},
// 	// // 	// my_anchor_project.ProgramID,
// 	// // )
// 	// // ix, err := my_anchor_project.NewUpdateDataInstruction("test-data-new", dataAccountAddress)
// 	// require.NoError(t, err)

// 	// // ix2, err := my_anchor_project.NewUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress)
// 	// // require.NoError(t, err)

// 	// // res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix, ix2}, pk, rpc.CommitmentConfirmed)
// 	// res, err := common.SendAndConfirm(context.Background(), solanaClient, []solana.Instruction{ix}, pk, rpc.CommitmentConfirmed)

// 	// require.NoError(t, err)
// 	// fmt.Println("res", res.Meta.LogMessages)

// 	// // output, err := common.ExtractTypedReturnValue(context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String(), func(b []byte) string {
// 	// // 	require.Len(t, b, int(binary.LittleEndian.Uint32(b[:4]))+4) // the first 4 bytes just encodes the length
// 	// // 	return string(b[4:])
// 	// // })
// 	// require.NoError(t, err)
// 	// fmt.Println("output", output)

// 	// output2, err := common.ExtractAnchorTypedReturnValue[my_anchor_project.UpdateResponse](context.Background(), res.Meta.LogMessages, my_anchor_project.ProgramID.String())
// 	// require.NoError(t, err)
// 	// fmt.Println("output2", output2)

// 	// output3, err := my_anchor_project.SendUpdateDataInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
// 	// require.NoError(t, err)
// 	// fmt.Println("output3", output3)

// 	// output4, err := my_anchor_project.SendUpdateDataWithTypedReturnInstruction("test-data-new", dataAccountAddress, solanaClient, pk, rpc.CommitmentConfirmed)
// 	// require.NoError(t, err)
// 	// fmt.Println("output4", output4.Data)
// }

/*
anchor-go \
  --idl /Users/yashvardhan/cre-client-program/my-project/target/idl/data_storage.json \
  --output /Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/data_storage \
  --program-id ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL \
  --no-go-mod

*/
