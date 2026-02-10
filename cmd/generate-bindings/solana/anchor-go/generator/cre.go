package generator

import (
	"encoding/json"
	"fmt"

	"github.com/dave/jennifer/jen"
	. "github.com/dave/jennifer/jen"
	"github.com/gagliardetto/anchor-go/tools"
)

// func (c *Codec) Decode<name>(data []byte) (*<name>, error) {
func creDecodeAccountFn(name string) Code {
	return Func().
		Params(Id("c").Op("*").Id("Codec")).
		Id("Decode"+name).
		Params(Id("data").Index().Byte()).
		Params(Op("*").Id(name), Error()).
		Block(Return(Id("ParseAccount_" + name).Call(Id("data"))))
}

//	func (c *Codec) Encode<StructName>Struct(in <StructName>) ([]byte, error) {
//		return in.Marshal()
//	}
func creGenerateCodecEncoderForTypes(exportedAccountName string) Code {
	return Func().
		Params(Id("c").Op("*").Id("Codec")).
		Id("Encode"+exportedAccountName+"Struct").
		Params(Id("in").Id(exportedAccountName)).
		Params(Index().Byte(), Error()).
		Block(Return(Id("in").Dot("Marshal").Call()))
}

// if err block
//
//		return cre.PromiseFromResult[*<StructName>](nil, err)
//	}
func creWriteReportErrorBlock() Code {
	code := Empty()
	code.If(Id("err").Op("!=").Nil()).Block(
		Return(
			Qual(PkgCRE, "PromiseFromResult").Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply")).Call(
				Nil(), Id("err"),
			)))
	code.Line().Line()
	return code
}

// func (c *DataStorage) WriteReportFrom<StructName>(runtime cre.Runtime, input <StructName>, accountList []solanago.PublicKey) cre.Promise[*solana.WriteReportReply] {
func creWriteReportFromStructs(exportedAccountName string, g *Generator) Code {
	code := Empty()
	declarerName := newWriteReportFromInstructionFuncName(exportedAccountName)
	code.Func().
		Params(Id("c").Op("*").Id(tools.ToCamelUpper(g.options.Package))). // method receiver
		Id(declarerName).
		// params
		Params(
			ListMultiline(func(p *Group) {
				p.Id("runtime").Qual(PkgCRE, "Runtime")
				p.Id("input").Id(exportedAccountName)
				p.Id("remainingAccounts").Index().Op("*").Qual(PkgSolanaCre, "AccountMeta")
			}),
		).
		// return type
		Params(Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply"))).
		BlockFunc(func(block *Group) {
			// encoded, err := c.Codec.Encode<exportedAccountName>Struct(input)
			block.List(Id("encodedInput"), Id("err")).Op(":=").
				Id("c").Dot("Codec").Dot("Encode" + exportedAccountName + "Struct").Call(Id("input"))

			// if err block
			block.Add(creWriteReportErrorBlock())

			// encodedAccountList, err := EncodeAccountList(accountList)
			block.Id("encodedAccountList").Op(":=").
				Qual(PkgBindings, "CalculateAccountsHash").Call(Id("remainingAccounts")).Line()

			// fwdReport := ForwarderReport{Payload: encodedInput, AccountHash: encodedAccountList}
			block.Id("fwdReport").Op(":=").Qual(PkgBindings, "ForwarderReport").Values(Dict{
				Id("Payload"):     Id("encodedInput"),
				Id("AccountHash"): Id("encodedAccountList"),
			})

			// encodedFwdReport, err := fwdReport.Marshal()
			block.List(Id("encodedFwdReport"), Id("err")).Op(":=").Id("fwdReport").Dot("Marshal").Call()

			// if err block
			block.Add(creWriteReportErrorBlock())

			// promise := runtime.GenerateReport(&pb2.ReportRequest{ ... })
			block.Id("promise").Op(":=").Id("runtime").Dot("GenerateReport").Call(
				Op("&").Qual(PkgPbSdk, "ReportRequest").Values(Dict{
					Id("EncodedPayload"): Id("encodedFwdReport"),
					Id("EncoderName"):    Lit("solana"),
					Id("SigningAlgo"):    Lit("ed25519"),
					Id("HashingAlgo"):    Lit("sha256"),
				}),
			).Line()

			// typedAccountList := make([]solana.PublicKey, len(accountList))
			// block.Id("typedAccountList").Op(":=").
			// 	Id("make").Call(
			// 	Index().Qual(PkgSolanaCre, "PublicKey"),
			// 	Id("len").Call(Id("accountList")),
			// )

			// // for i, account := range accountList {
			// //     typedAccountList[i] = solana.PublicKey(account)
			// // }
			// block.For(
			// 	List(Id("i"), Id("account")).Op(":=").Range().Id("accountList"),
			// ).Block(
			// 	Id("typedAccountList").Index(Id("i")).Op("=").
			// 		Qual(PkgSolanaCre, "PublicKey").Call(Id("account")),
			// ).Line()

			//return cre.ThenPromise(promise, func(report *cre.Report) cre.Promise[*solana.WriteReportReply] {
			// 	return c.client.WriteReport(runtime, &solana.WriteCreReportRequest{
			// 		AccountList: typedAccountList,
			// 		Receiver:    ProgramID.Bytes(),
			// 		Report:      report,
			// 	})
			// })
			block.Return(
				Qual(PkgCRE, "ThenPromise").Call(
					Id("promise"),
					creWriteReportFromStructsLambda(),
				),
			)
		})
	return code
}

func creWriteReportFromStructsLambda() *Statement {
	// func(report *cre.Report) cre.Promise[*solana.WriteReportReply] {
	// 	return c.client.WriteReport(runtime, &solana.WriteCreReportRequest{
	// 		AccountList: typedAccountList,
	// 		Receiver:    ProgramID.Bytes(),
	// 		Report:      report,
	// 	})
	// }
	return Func().
		Params(Id("report").Op("*").Qual(PkgCRE, "Report")).
		Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply")).
		Block(
			Return(
				Id("c").Dot("client").Dot("WriteReport").Call(
					Id("runtime"),
					Op("&").Qual(PkgSolanaCre, "WriteCreReportRequest").Values(jen.Dict{
						Id("Receiver"):          Id("ProgramID").Dot("Bytes").Call(),
						Id("Report"):            Id("report"),
						Id("RemainingAccounts"): Id("remainingAccounts"),
					}),
				),
			),
		)
}

// genfile_constructor generates the file `constructor.go`.
func (g *Generator) genfile_constructor() (*OutputFile, error) {
	file := NewFile(g.options.Package)
	file.HeaderComment("Code generated by https://github.com/gagliardetto/anchor-go. DO NOT EDIT.")
	file.HeaderComment("This file contains the constructor for the program.")

	{
		// idl string
		code := newStatement()
		idlData, err := json.Marshal(g.idl)
		if err != nil {
			return nil, fmt.Errorf("error reading IDL file: %w", err)
		}
		code.Var().Id("IDL").Op("=").Lit(string(idlData))
		file.Add(code)
		code.Line()

		// contract type
		code = newStatement()
		code.Type().Id(tools.ToCamelUpper(g.options.Package)).Struct(
			Id("client").Op("*").Qual(PkgSolanaCre, "Client"),
			Id("Codec").Id(tools.ToCamelUpper(g.options.Package)+"Codec"),
		)
		code.Line()
		file.Add(code)
		code.Line()

		// codec type
		code = newStatement()
		code.Type().Id("Codec").Struct()
		code.Line()
		file.Add(code)

		// new constructor
		code = newStatement()
		code.Func().
			Id("New"+tools.ToCamelUpper(g.options.Package)).
			Params(
				Id("client").Op("*").Qual(PkgSolanaCre, "Client"),
			).
			Params(
				Op("*").Id(tools.ToCamelUpper(g.options.Package)), Error(),
			).
			Block(
				// return &DataStorage{ Codec: &Codec{}, IdlTypes: &idlTypes.IdlTypeDefSlice, client: client }, nil
				Return(
					Op("&").Id(tools.ToCamelUpper(g.options.Package)).Values(Dict{
						Id("Codec"):  Op("&").Id("Codec").Values(),
						Id("client"): Id("client"),
					}),
					Nil(),
				),
			)
		file.Add(code)
		code.Line()

		methods, err := g.generateCodecMethods()
		if err != nil {
			return nil, err
		}

		// Codec interface
		code = newStatement()
		code.Type().Id(tools.ToCamelUpper(g.options.Package) + "Codec").Interface(methods...)
		file.Add(code)
		code.Line()
	}

	return &OutputFile{
		Name: "constructor.go",
		File: file,
	}, nil
}

func getAccountInfoLambda() *Statement {
	// func(bn uint64) cre.Promise[*solana.GetAccountInfoWithOptsReply] {
	// 	return c.client.GetAccountInfoWithOpts(runtime, &solana.GetAccountInfoWithOptsRequest{
	// 		Account: types.PublicKey(accountAddress),
	// 		Opts:    &solana.GetAccountInfoOpts{MinContextSlot: &bn},
	// 	})
	// }
	return Func().
		Params(Id("bn").Uint64()).
		Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "GetAccountInfoWithOptsReply")).
		Block(
			Return(
				Id("c").Dot("client").Dot("GetAccountInfoWithOpts").Call(
					Id("runtime"),
					Op("&").Qual(PkgSolanaCre, "GetAccountInfoWithOptsRequest").Values(Dict{
						Id("Account"): Id("accountAddress").Dot("Bytes").Call(),
						Id("Opts"): Op("&").Qual(PkgSolanaCre, "GetAccountInfoOpts").Values(Dict{
							Id("MinContextSlot"): Id("bn"),
						}),
					}),
				),
			),
		)
}

func parseAccountLambda(name string) *Statement {
	// func(response *solana.GetAccountInfoWithOptsReply) (*DataAccount, error) {
	// 	return ParseAccount_DataAccount(response.Value.Data.AsDecodedBinary)
	// }
	return Func().
		Params(Id("response").Op("*").Qual(PkgSolanaCre, "GetAccountInfoWithOptsReply")).
		Params(Op("*").Id(name), Error()).
		Block(
			Return(
				Id("ParseAccount_" + name).Call(
					Id("response").Dot("Value").Dot("Data").Dot("GetRaw").Call(),
				),
			),
		)
}

func (g *Generator) generateCodecAccountMethods() ([]Code, error) {
	accountMethods := make([]Code, 0, len(g.idl.Accounts))
	for _, acc := range g.idl.Accounts {
		methodName := "Decode" + acc.Name
		m := Id(methodName).
			Params(Id("data").Index().Byte()). // ([]byte)
			Params(
				Op("*").Id(acc.Name), // (*DataAccount)
				Error(),              // error
			)

		accountMethods = append(accountMethods, m)
	}

	return accountMethods, nil
}

func (g *Generator) generateCodecStructMethod() ([]Code, error) {
	structMethods := make([]Code, 0, len(g.idl.Types))
	for _, typ := range g.idl.Types {
		methodName := "Encode" + typ.Name + "Struct"
		m := Id(methodName).
			Params(
				Id("in").Id(typ.Name), // e.g., AccessLogged / DataAccount / ...
			).
			Params(
				Index().Byte(), // []byte
				Error(),        // error
			)
		structMethods = append(structMethods, m)
	}
	return structMethods, nil
}

func (g *Generator) generateCodecMethods() ([]Code, error) {
	accountMethods, err := g.generateCodecAccountMethods()
	if err != nil {
		return nil, err
	}

	structMethods, err := g.generateCodecStructMethod()
	if err != nil {
		return nil, err
	}
	return append(accountMethods, structMethods...), nil
}
