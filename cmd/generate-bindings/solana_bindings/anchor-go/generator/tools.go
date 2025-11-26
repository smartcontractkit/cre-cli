package generator

import (
	"os"
	"path"

	. "github.com/dave/jennifer/jen"
)

const (
	PkgBinary   = "github.com/gagliardetto/binary"
	PkgCRE      = "github.com/smartcontractkit/cre-sdk-go/cre"
	PkgPb       = "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	PkgPb2      = "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	PkgSolanaGo = "github.com/gagliardetto/solana-go"

	PkgSolanaGoText   = "github.com/gagliardetto/solana-go/text"
	PkgAnchorGoErrors = "github.com/gagliardetto/anchor-go/errors"
	PkgBig            = "math/big"
	// TODO: use or remove this:
	PkgTreeout        = "github.com/gagliardetto/treeout"
	PkgFormat         = "github.com/gagliardetto/solana-go/text/format"
	PkgGoFuzz         = "github.com/gagliardetto/gofuzz"
	PkgTestifyRequire = "github.com/stretchr/testify/require"
	PkgSolanaCre      = "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/capabilities/blockchain/solana"
	PkgRealSolanaCre  = "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
	PkgBindings       = "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/capabilities/blockchain/solana/bindings"
	PkgSolanaTypes    = "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/types"
	PkgIdl            = "github.com/gagliardetto/anchor-go/idl"
	PkgAnchorIdlCodec = "github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings/cre-sdk-go/anchorcodec"
	PkgJson           = "encoding/json"
)

func WriteFile(outDir string, assetFileName string, file *File) error {
	// Save Go assets:
	assetFilepath := path.Join(outDir, assetFileName)

	// Create file Golang file:
	goFile, err := os.Create(assetFilepath)
	if err != nil {
		panic(err)
	}
	defer goFile.Close()

	// Write generated Golang to file:
	return file.Render(goFile)
}

func DoGroup(f func(*Group)) *Statement {
	g := &Group{}
	g.CustomFunc(Options{
		Multi: false,
	}, f)
	s := newStatement()
	*s = append(*s, g)
	return s
}

func DoGroupMultiline(f func(*Group)) *Statement {
	g := &Group{}
	g.CustomFunc(Options{
		Multi: true,
	}, f)
	s := newStatement()
	*s = append(*s, g)
	return s
}

func ListMultiline(f func(*Group)) *Statement {
	g := &Group{}
	g.CustomFunc(Options{
		Multi:     true,
		Separator: ",",
		Open:      "",
		Close:     " ",
	}, f)
	s := newStatement()
	*s = append(*s, g)
	return s
}

func newStatement() *Statement {
	return &Statement{}
}
