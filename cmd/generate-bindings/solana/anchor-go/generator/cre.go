// This file contains all the cre specific code for the generator.
// The other files are copied from https://github.com/gagliardetto/anchor-go/blob/main/generator/
// They simply call functions in this file.
//
//nolint:all
package generator

import (
	"encoding/json"
	"fmt"

	. "github.com/dave/jennifer/jen"
	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/tools"
)

const (
	// cre
	PkgBig         = "math/big"
	PkgIdl         = "github.com/gagliardetto/anchor-go/idl"
	PkgJson        = "encoding/json"
	PkgCRE         = "github.com/smartcontractkit/cre-sdk-go/cre"
	PkgPb          = "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	PkgPbSdk       = "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	PkgSolanaCre   = "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
	PkgBindings    = "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana/bindings"
	PkgSolanaTypes = "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana/cre-sdk-go/types"
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

func creWriteReportFromStructs(exportedAccountName string, g *Generator) Code {
	code := Empty()
	declarerName := newWriteReportFromInstructionFuncName(exportedAccountName)
	code.Commentf("%s encodes the input struct, hashes the provided accounts,", declarerName)
	code.Comment("generates a signed report, and submits it via WriteReport.")
	code.Comment("")
	code.Comment("remainingAccounts must follow the keystone-forwarder account layout:")
	code.Comment("  - Index 0: forwarderState – the forwarder program's state account.")
	code.Comment("  - Index 1: forwarderAuthority – PDA derived from seeds")
	code.Comment("    [\"forwarder\", forwarderState, receiverProgram] under the forwarder program ID.")
	code.Comment("  - Index 2+: receiver-specific accounts required by the target program.")
	code.Comment("")
	code.Comment("The full slice is hashed (via CalculateAccountsHash) into the report and forwarded")
	code.Comment("as WriteCreReportRequest.RemainingAccounts. The on-chain forwarder strips indices 0 and 1")
	code.Comment("before CPI-ing into the receiver, so they must be present and correctly ordered.")
	code.Line()
	code.Func().
		Params(Id("c").Op("*").Id(tools.ToCamelUpper(g.options.Package))). // method receiver
		Id(declarerName).
		// params
		Params(
			ListMultiline(func(p *Group) {
				p.Id("runtime").Qual(PkgCRE, "Runtime")
				p.Id("input").Id(exportedAccountName)
				p.Id("remainingAccounts").Index().Op("*").Qual(PkgSolanaCre, "AccountMeta")
				p.Id("computeConfig").Op("*").Qual(PkgSolanaCre, "ComputeConfig")
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
					Id("SigningAlgo"):    Lit("ecdsa"),
					Id("HashingAlgo"):    Lit("keccak256"),
				}),
			).Line()

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

func creEncodeBorshVecU32() Code {
	st := Empty()
	st.Comment(`EncodeBorshVecU32 returns Anchor/Borsh encoding of a Vec whose elements are opaque byte payloads.`)
	st.Comment(`Each [][]byte element must already be fully serialized for one Vec item on the wire.`)
	st.Comment(`Layout: little-endian u32 length followed by concatenated element payloads.`)
	st.Line()
	st.Func().
		Id("EncodeBorshVecU32").
		Params(Id("elements").Index().Index().Byte()).
		Params(Index().Byte(), Error()).
		BlockFunc(func(b *Group) {
			b.Id("buf").Op(":=").Qual("bytes", "NewBuffer").Call(Nil())
			b.If(
				Err().Op(":=").Qual("encoding/binary", "Write").Call(
					Id("buf"),
					Qual("encoding/binary", "LittleEndian"),
					Id("uint32").Call(Len(Id("elements"))),
				),
				Err().Op("!=").Nil(),
			).Block(Return(Nil(), Err()))
			b.For(Id("_").Op(",").Id("elem").Op(":=").Range().Id("elements")).Block(
				List(Id("_"), Err()).Op(":=").Id("buf").Dot("Write").Call(Id("elem")),
				If(Err().Op("!=").Nil()).Block(
					Return(Nil(), Err()),
				),
			)
			b.Return(Id("buf").Dot("Bytes").Call(), Nil())
		})
	return st
}

// WriteReportFromBorshEncodedVec forwards a CRE report whose inner payload is EncodeBorshVecU32(elementPayloads).
func creWriteReportFromBorshEncodedVec(g *Generator) Code {
	pkg := tools.ToCamelUpper(g.options.Package)
	code := Empty()
	code.Comment(`WriteReportFromBorshEncodedVec publishes through the CRE signer using a forwarder payload built from`)
	code.Comment(`Borsh Vec<byte…> semantics (EncodeBorshVecU32). Compose each elementPayload for your program (e.g. one encoded struct per row).`)
	code.Comment(`Pass computeConfig = nil to use the host default Solana compute budget.`)
	code.Line()
	code.Func().
		Params(Id("c").Op("*").Id(pkg)).
		Id("WriteReportFromBorshEncodedVec").
		Params(ListFunc(func(pl *Group) {
			pl.Id("runtime").Qual(PkgCRE, "Runtime")
			pl.Id("elementPayloads").Index().Index().Byte()
			pl.Id("remainingAccounts").Index().Op("*").Qual(PkgSolanaCre, "AccountMeta")
			pl.Id("computeConfig").Op("*").Qual(PkgSolanaCre, "ComputeConfig")
		})).
		Params(Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply"))).
		BlockFunc(func(block *Group) {
			block.List(Id("payload"), Id("err")).Op(":=").Id("EncodeBorshVecU32").Call(Id("elementPayloads"))
			block.Add(creWriteReportErrorBlock())
			block.Id("encodedAccountList").Op(":=").Qual(PkgBindings, "CalculateAccountsHash").Call(Id("remainingAccounts"))
			block.Id("fwdReport").Op(":=").Qual(PkgBindings, "ForwarderReport").Values(Dict{
				Id("AccountHash"): Id("encodedAccountList"),
				Id("Payload"):     Id("payload"),
			})
			block.List(Id("encodedFwdReport"), Id("err")).Op(":=").Id("fwdReport").Dot("Marshal").Call()
			block.Add(creWriteReportErrorBlock())
			block.Id("promise").Op(":=").Id("runtime").Dot("GenerateReport").Call(
				Op("&").Qual(PkgPbSdk, "ReportRequest").Values(Dict{
					Id("EncodedPayload"): Id("encodedFwdReport"),
					Id("EncoderName"):    Lit("solana"),
					Id("SigningAlgo"):    Lit("ecdsa"),
					Id("HashingAlgo"):    Lit("keccak256"),
				}),
			).Line()
			block.Return(Qual(PkgCRE, "ThenPromise").Call(Id("promise"), creWriteReportFromStructsLambda()))
		})
	return code
}

// creWriteReportFromStructsSlice emits:
//
//	func (c *<Pkg>) WriteReportFrom<StructName>s(runtime cre.Runtime, inputs []<StructName>, remainingAccounts []*solana.AccountMeta, computeConfig *solana.ComputeConfig) cre.Promise[*solana.WriteReportReply] {
//	    elements := make([][]byte, len(inputs))
//	    for i, input := range inputs {
//	        encoded, err := c.Codec.Encode<StructName>Struct(input)
//	        if err != nil { return cre.PromiseFromResult[*solana.WriteReportReply](nil, err) }
//	        elements[i] = encoded
//	    }
//	    return c.WriteReportFromBorshEncodedVec(runtime, elements, remainingAccounts, computeConfig)
//	}
func creWriteReportFromStructsSlice(exportedStructName string, g *Generator) Code {
	pkg := tools.ToCamelUpper(g.options.Package)
	declarerName := newWriteReportFromInstructionFuncName(exportedStructName) + "s"
	return Func().
		Params(Id("c").Op("*").Id(pkg)).
		Id(declarerName).
		Params(ListMultiline(func(p *Group) {
			p.Id("runtime").Qual(PkgCRE, "Runtime")
			p.Id("inputs").Index().Id(exportedStructName)
			p.Id("remainingAccounts").Index().Op("*").Qual(PkgSolanaCre, "AccountMeta")
			p.Id("computeConfig").Op("*").Qual(PkgSolanaCre, "ComputeConfig")
		})).
		Params(Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply"))).
		BlockFunc(func(block *Group) {
			block.Id("elements").Op(":=").Make(Index().Index().Byte(), Len(Id("inputs")))
			block.For(Id("i").Op(",").Id("input").Op(":=").Range().Id("inputs")).Block(
				List(Id("encoded"), Err()).Op(":=").
					Id("c").Dot("Codec").Dot("Encode"+exportedStructName+"Struct").Call(Id("input")),
				If(Err().Op("!=").Nil()).Block(
					Return(Qual(PkgCRE, "PromiseFromResult").
						Types(Op("*").Qual(PkgSolanaCre, "WriteReportReply")).
						Call(Nil(), Err())),
				),
				Id("elements").Index(Id("i")).Op("=").Id("encoded"),
			)
			block.Return(Id("c").Dot("WriteReportFromBorshEncodedVec").Call(
				Id("runtime"),
				Id("elements"),
				Id("remainingAccounts"),
				Id("computeConfig"),
			))
		})
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
					Op("&").Qual(PkgSolanaCre, "WriteCreReportRequest").Values(Dict{
						Id("Receiver"):          Id("ProgramID").Dot("Bytes").Call(),
						Id("Report"):            Id("report"),
						Id("RemainingAccounts"): Id("remainingAccounts"),
						Id("ComputeConfig"):     Id("computeConfig"),
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

		file.Add(creEncodeBorshVecU32())
		code.Line()
		file.Add(creWriteReportFromBorshEncodedVec(g))
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

func (g *Generator) generateCodecAccountMethods() ([]Code, error) {
	accountMethods := make([]Code, 0, len(g.idl.Accounts))
	for _, acc := range g.idl.Accounts {
		exportedName := tools.ToCamelUpper(acc.Name)
		methodName := "Decode" + exportedName
		m := Id(methodName).
			Params(Id("data").Index().Byte()). // ([]byte)
			Params(
				Op("*").Id(exportedName), // (*DataAccount)
				Error(),                  // error
			)

		accountMethods = append(accountMethods, m)
	}

	return accountMethods, nil
}

func (g *Generator) generateCodecStructMethod() ([]Code, error) {
	structMethods := make([]Code, 0, len(g.idl.Types))
	for _, typ := range g.idl.Types {
		exportedName := tools.ToCamelUpper(typ.Name)
		methodName := "Encode" + exportedName + "Struct"
		if _, isEnum := typ.Ty.(*idl.IdlTypeDefTyEnum); isEnum {
			continue
		}
		m := Id(methodName).
			Params(
				Id("in").Id(exportedName), // e.g., AccessLogged / DataAccount / ...
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
