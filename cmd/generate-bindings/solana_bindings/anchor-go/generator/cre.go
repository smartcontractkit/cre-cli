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

// func (c *DataStorage) ReadAccount_DataAccount(runtime cre.Runtime, accountAddress solanago.PublicKey, blockNumber *big.Int) cre.Promise[*<StructName>] {
func creReadAccountFn(name string, g *Generator) Code {
	code := Func().
		Params(Id("c").Op("*").Id(tools.ToCamelUpper(g.options.Package))). // method receiver
		Id("ReadAccount_" + name).
		Params(
			ListMultiline(
				func(paramsCode *Group) {
					paramsCode.Id("runtime").Qual(PkgCRE, "Runtime")
					paramsCode.Id("accountAddress").Qual(PkgSolanaGo, "PublicKey")
					paramsCode.Id("blockNumber").Op("*").Qual(PkgBig, "Int")
				},
			),
		).
		Params(
			Qual(PkgCRE, "Promise").Types(Op("*").Id(name)),
		).
		BlockFunc(func(block *Group) {
			block.Comment("cre account read")
			// bn := cre.PromiseFromResult(uint64(blockNumber.Int64()), nil)
			block.Id("bn").Op(":=").Qual(PkgCRE, "PromiseFromResult").Call(
				Id("uint64").Call(Id("blockNumber").Dot("Int64").Call()),
				Nil(),
			)
			// promise := cre.ThenPromise(bn, func(bn uint64) cre.Promise[*solana.GetAccountInfoReply] {
			// 	return c.client.GetAccountInfoWithOpts(runtime, &solana.GetAccountInfoRequest{
			// 		Account: types.PublicKey(accountAddress),
			// 		Opts:    &solana.GetAccountInfoOpts{MinContextSlot: &bn},
			// 	})
			// })
			block.Id("promise").Op(":=").Qual(PkgCRE, "ThenPromise").Call(
				Id("bn"),
				getAccountInfoLambda(),
			)
			// return cre.Then(promise, func(response *solana.GetAccountInfoReply) (*DataAccount, error) {
			// 	return ParseAccount_DataAccount(response.Value.Data.AsDecodedBinary)
			// })
			block.Return(
				Qual(PkgCRE, "Then").Call(
					Id("promise"),
					parseAccountLambda(name),
				),
			)
		})
	return code
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
				p.Id("accountList").Index().Qual(PkgSolanaGo, "PublicKey")
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
			block.List(Id("encodedAccountList"), Id("err")).Op(":=").
				Id("EncodeAccountList").Call(Id("accountList"))

			// if err block
			block.Add(creWriteReportErrorBlock())

			// fwdReport := ForwarderReport{Payload: encodedInput, AccountHash: encodedAccountList}
			block.Id("fwdReport").Op(":=").Qual(PkgSolanaCre, "ForwarderReport").Values(Dict{
				Id("Payload"):     Id("encodedInput"),
				Id("AccountHash"): Id("encodedAccountList"),
			})

			// encodedFwdReport, err := fwdReport.Marshal()
			block.List(Id("encodedFwdReport"), Id("err")).Op(":=").Id("fwdReport").Dot("Marshal").Call()

			// if err block
			block.Add(creWriteReportErrorBlock())

			// promise := runtime.GenerateReport(&pb2.ReportRequest{ ... })
			block.Id("promise").Op(":=").Id("runtime").Dot("GenerateReport").Call(
				Op("&").Qual(PkgPb2, "ReportRequest").Values(Dict{
					Id("EncodedPayload"): Id("encodedFwdReport"),
					Id("EncoderName"):    Lit("solana"),
					Id("SigningAlgo"):    Lit("ed25519"),
					Id("HashingAlgo"):    Lit("sha256"),
				}),
			).Line()

			// typedAccountList := make([]solana.PublicKey, len(accountList))
			block.Id("typedAccountList").Op(":=").
				Id("make").Call(
				Index().Qual(PkgSolanaCre, "PublicKey"),
				Id("len").Call(Id("accountList")),
			)

			// for i, account := range accountList {
			//     typedAccountList[i] = solana.PublicKey(account)
			// }
			block.For(
				List(Id("i"), Id("account")).Op(":=").Range().Id("accountList"),
			).Block(
				Id("typedAccountList").Index(Id("i")).Op("=").
					Qual(PkgSolanaCre, "PublicKey").Call(Id("account")),
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
						Id("Receiver"):    Id("ProgramID").Dot("Bytes").Call(),
						Id("Report"):      Id("report"),
						Id("AccountList"): Id("typedAccountList"),
					}),
				),
			),
		)
}

// func (c *Codec) Decode<name>(event *solana.Log) (*<name>, error) {
func creDecodeEventFn(name string) Code {
	return Func().
		Params(Id("c").Op("*").Id("Codec")). // method receiver
		Id("Decode"+name).
		Params(Id("event").Op("*").Qual(PkgSolanaCre, "Log")).
		Params(Op("*").Id(name), Error()).
		BlockFunc(func(block *Group) {
			// res, err := ParseEvent_<name>(event.Data)
			block.List(Id("res"), Id("err")).Op(":=").Id("ParseEvent_" + name).Call(
				Id("event").Dot("Data"),
			)
			block.Add(nilErrBlock())
			block.Return(Id("res"), Nil())
		}).Line().Line()
}

//	type <name>Trigger struct {
//		cre.Trigger[*solana.Log, *solana.Log]
//		contract *<package name>
//	}
func creTriggerType(name string, g *Generator) Code {
	return Type().Id(name+"Trigger").
		Struct(
			Qual(PkgCRE, "Trigger"). // embedded generic type
							Types(
					Op("*").Qual(PkgSolanaCre, "Log"),
					Op("*").Qual(PkgSolanaCre, "Log"),
				),
			Id("contract").Op("*").Id(tools.ToCamelUpper(g.options.Package)),
		).Line().Line()
}

// func (t *AccessLoggedTrigger) Adapt(l *solana.Log) (*bindings.DecodedLog[AccessLogged], error) {
func creLogTriggerAdaptFn(name string) Code {
	return Func().
		Params(Id("t").Op("*").Id(name+"Trigger")). // receiver (*DataUpdatedTrigger)
		Id("Adapt").
		Params(Id("l").Op("*").Qual(PkgSolanaCre, "Log")).
		Params(
			Op("*").Qual(PkgBindings, "DecodedLog").Types(Id(name)), // return type
			Error(),
		).
		Block(
			// decoded, err := t.contract.Codec.Decode<name>(l)
			List(Id("decoded"), Id("err")).Op(":=").Id("t").Dot("contract").Dot("Codec").Dot("Decode"+name).Call(Id("l")),
			// if err != nil { return nil, err }
			Add(nilErrBlock()),
			// return &bindings.DecodedLog<name>{ Log: l, Data: *decoded }
			Return(
				Op("&").Qual(PkgBindings, "DecodedLog").Types(Id(name)).Values(Dict{
					Id("Log"):  Id("l"),
					Id("Data"): Op("*").Id("decoded"),
				}),
				Nil(),
			),
		).Line().Line()
}

// func (c *pkgName) LogTrigger_<name>(chainSelector uint64, subKeyPathAndValue []solana.SubKeyPathAndFilter) (cre.Trigger[*solana.Log, *bindings.DecodedLog[<name>]], error) {
func creLogTriggerFunc(name string, g *Generator) Code {
	return Func().
		Params(Id("c").Op("*").Id(tools.ToCamelUpper(g.options.Package))). // method receiver
		Id("LogTrigger_"+name).
		Params(
			Id("chainSelector").Uint64(),
			Id("subKeyPathAndValue").Index().Qual(PkgSolanaCre, "SubKeyPathAndFilter"),
		).
		Params(
			Qual(PkgCRE, "Trigger").Types(
				Op("*").Qual(PkgSolanaCre, "Log"),
				Op("*").Qual(PkgBindings, "DecodedLog").Types(Id(name)),
			),
			Error(),
		).
		BlockFunc(func(b *jen.Group) {
			// eventIdl := types.GetIdlEvent(c.IdlTypes, "<Event>")
			b.List(Id("eventIdl"), Id("err")).Op(":=").Qual(PkgSolanaTypes, "GetIdlEvent").Call(
				Id("c").Dot("IdlTypes"),
				Lit(name),
			)
			b.Add(nilErrBlock())

			// if len(subKeyPathAndValue) > 4 { return nil, fmt.Errorf(...) }
			b.If(Len(Id("subKeyPathAndValue")).Op(">").Lit(4)).Block(
				Return(
					Nil(),
					Qual("fmt", "Errorf").Call(
						Lit("too many subkey path and value pairs: %d"),
						Len(Id("subKeyPathAndValue")),
					),
				),
			)

			// subKeyPaths, subKeyFilters, err := bindings.ValidateSubKeyPathAndValue[<Event>](subKeyPathAndValue)
			b.List(
				Id("subKeyPaths"),
				Id("subKeyFilters"),
				Id("err"),
			).Op(":=").Qual(PkgBindings, "ValidateSubKeyPathAndValue").
				Types(Id(name)).
				Call(Id("subKeyPathAndValue"))

			b.If(Id("err").Op("!=").Nil()).Block(
				Return(
					Nil(),
					Qual("fmt", "Errorf").Call(
						Lit("failed to validate subkey path and value: %w"),
						Id("err"),
					),
				),
			)

			// rawTrigger := solana.LogTrigger(chainSelector, &solana.FilterLogTriggerRequest{ ... })
			b.Id("rawTrigger").Op(":=").Qual(PkgSolanaCre, "LogTrigger").Call(
				Id("chainSelector"),
				Op("&").Qual(PkgSolanaCre, "FilterLogTriggerRequest").Values(jen.Dict{
					Id("Address"):       Qual(PkgSolanaTypes, "PublicKey").Call(Id("ProgramID")),
					Id("EventName"):     Lit(name),
					Id("EventSig"):      Id("Event_" + name),
					Id("EventIdl"):      Id("eventIdl"),
					Id("SubkeyPaths"):   Id("subKeyPaths"),
					Id("SubkeyFilters"): Id("subKeyFilters"),
				}),
			)

			// return &<Event>Trigger{ Trigger: rawTrigger }, nil
			b.Return(
				Op("&").Id(name+"Trigger").Values(jen.Dict{
					Id("Trigger"):  Id("rawTrigger"),
					Id("contract"): Id("c"),
				}),
				Nil(),
			)
		}).Line().Line()
}

func nilErrBlock() Code {
	return If(Id("err").Op("!=").Nil()).Block(
		Return(Nil(), Id("err")),
	)
}

func creEventFuncs(name string, g *Generator) Code {
	code := Empty()
	// event decode func
	code.Add(creDecodeEventFn(name))

	// trigger type
	code.Add(creTriggerType(name, g))

	// Adapt func
	code.Add(creLogTriggerAdaptFn(name))

	// Log trigger func
	code.Add(creLogTriggerFunc(name, g))

	return code
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
			Id("IdlTypes").Op("*").Qual(PkgAnchorIdlCodec, "IdlTypeDefSlice"),
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
				// type idlTypesStruct struct { anchorcodec.IdlTypeDefSlice `json:"types"` }
				Type().Id("idlTypesStruct").Struct(
					Qual(PkgAnchorIdlCodec, "IdlTypeDefSlice").
						Tag(map[string]string{"json": "types"}),
				),

				// var idlTypes idlTypesStruct
				Var().Id("idlTypes").Id("idlTypesStruct"),

				// err := json.Unmarshal([]byte(IDL), &idlTypes)
				Id("err").Op(":=").Qual(PkgJson, "Unmarshal").Call(
					Index().Byte().Parens(Id("IDL")),
					Op("&").Id("idlTypes"),
				),

				// if err != nil { return nil, err }
				If(Err().Op("!=").Nil()).Block(
					Return(Nil(), Err()),
				),

				// return &DataStorage{ Codec: &Codec{}, IdlTypes: &idlTypes.IdlTypeDefSlice, client: client }, nil
				Return(
					Op("&").Id(tools.ToCamelUpper(g.options.Package)).Values(Dict{
						Id("Codec"):    Op("&").Id("Codec").Values(),
						Id("IdlTypes"): Op("&").Id("idlTypes").Dot("IdlTypeDefSlice"),
						Id("client"):   Id("client"),
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

		// dummyForwarderCode(file)

		// dummy account encoding function
		code = newStatement()
		code.Func().
			Id("EncodeAccountList").
			Params(Id("accountList").Index().Qual(PkgSolanaGo, "PublicKey")).
			Params(Index(Lit(32)).Byte(), Error()).
			Block(Return(Index(Lit(32)).Byte().Values(), Nil()))
		file.Add(code)
		code.Line()
	}

	return &OutputFile{
		Name: "constructor.go",
		File: file,
	}, nil
}

func getAccountInfoLambda() *Statement {
	// func(bn uint64) cre.Promise[*solana.GetAccountInfoReply] {
	// 	return c.client.GetAccountInfoWithOpts(runtime, &solana.GetAccountInfoRequest{
	// 		Account: types.PublicKey(accountAddress),
	// 		Opts:    &solana.GetAccountInfoOpts{MinContextSlot: &bn},
	// 	})
	// }
	return Func().
		Params(Id("bn").Uint64()).
		Qual(PkgCRE, "Promise").Types(Op("*").Qual(PkgSolanaCre, "GetAccountInfoReply")).
		Block(
			Return(
				Id("c").Dot("client").Dot("GetAccountInfoWithOpts").Call(
					Id("runtime"),
					Op("&").Qual(PkgSolanaCre, "GetAccountInfoRequest").Values(Dict{
						Id("Account"): Qual(PkgSolanaTypes, "PublicKey").Call(Id("accountAddress")),
						Id("Opts"): Op("&").Qual(PkgSolanaCre, "GetAccountInfoOpts").Values(Dict{
							Id("MinContextSlot"): Op("&").Id("bn"),
						}),
					}),
				),
			),
		)
}

func parseAccountLambda(name string) *Statement {
	// func(response *solana.GetAccountInfoReply) (*DataAccount, error) {
	// 	return ParseAccount_DataAccount(response.Value.Data.AsDecodedBinary)
	// }
	return Func().
		Params(Id("response").Op("*").Qual(PkgSolanaCre, "GetAccountInfoReply")).
		Params(Op("*").Id(name), Error()).
		Block(
			Return(
				Id("ParseAccount_" + name).Call(
					Id("response").Dot("Value").Dot("Data").Dot("AsDecodedBinary"),
				),
			),
		)
}

func dummyForwarderCode(file *File) {
	code := newStatement()
	code.Type().Id("ForwarderReport").Struct(
		Id("AccountHash").Index(Lit(32)).Byte().Tag(map[string]string{"json": "account_hash"}),
		Id("Payload").Index().Byte().Tag(map[string]string{"json": "payload"}),
	)
	file.Add(code)
	code.Line()

	code = newStatement()
	code.Func().
		Params(Id("c").Op("*").Id("Codec")).
		Id("EncodeForwarderReportStruct").
		Params(Id("in").Id("ForwarderReport")).
		Params(Index().Byte(), Error()).
		Block(
			Return(Id("in").Dot("Marshal").Call()),
		)
	file.Add(code)
	code.Line()

	code = newStatement()
	code.Func().
		Params(Id("obj").Id("ForwarderReport")).
		Id("MarshalWithEncoder").
		Params(Id("encoder").Op("*").Qual(PkgBinary, "Encoder")).
		Params(Id("err").Error()).
		Block(
			Comment("Serialize `AccountHash`:"),
			Id("err").Op("=").Id("encoder").Dot("Encode").Call(Id("obj").Dot("AccountHash")),
			If(Id("err").Op("!=").Nil()).Block(
				Return(Qual(PkgAnchorGoErrors, "NewField").Call(Lit("AccountHash"), Id("err"))),
			),
			Comment("Serialize `Payload`:"),
			Id("err").Op("=").Id("encoder").Dot("Encode").Call(Id("obj").Dot("Payload")),
			If(Id("err").Op("!=").Nil()).Block(
				Return(Qual(PkgAnchorGoErrors, "NewField").Call(Lit("Payload"), Id("err"))),
			),
			Return(Nil()),
		)
	file.Add(code)
	code.Line()

	code = newStatement()
	code.Func().
		Params(Id("obj").Id("ForwarderReport")).
		Id("Marshal").
		Params().
		Params(Index().Byte(), Error()).
		Block(
			Id("buf").Op(":=").Qual("bytes", "NewBuffer").Call(Nil()),
			Id("encoder").Op(":=").Qual(PkgBinary, "NewBorshEncoder").Call(Id("buf")),
			Id("err").Op(":=").Id("obj").Dot("MarshalWithEncoder").Call(Id("encoder")),
			If(Id("err").Op("!=").Nil()).Block(
				Return(
					Nil(),
					Qual("fmt", "Errorf").Call(Lit("error while encoding ForwarderReport: %w"), Id("err")),
				),
			),
			Return(Id("buf").Dot("Bytes").Call(), Nil()),
		)
	file.Add(code)
	code.Line()
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

func (g *Generator) generateCodecEventMethods() ([]Code, error) {
	eventMethods := make([]Code, 0, len(g.idl.Events))
	for _, event := range g.idl.Events {
		methodName := "Decode" + event.Name
		m := Id(methodName).
			Params(Id("log").Op("*").Qual(PkgSolanaCre, "Log")).
			Params(
				Op("*").Id(event.Name),
				Error(),
			)

		eventMethods = append(eventMethods, m)
	}

	return eventMethods, nil
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
	eventMethods, err := g.generateCodecEventMethods()
	if err != nil {
		return nil, err
	}
	structMethods, err := g.generateCodecStructMethod()
	if err != nil {
		return nil, err
	}
	return append(append(accountMethods, eventMethods...), structMethods...), nil
}
