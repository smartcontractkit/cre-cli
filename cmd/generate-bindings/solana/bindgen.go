package solana

import (
	"bytes"
	"fmt"
	"go/token"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/tools"
	bin "github.com/gagliardetto/binary"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana/anchor-go/generator"
)

func GenerateBindings(
	pathToIdl string,
	programName string,
	outputDir string,
) error {
	if pathToIdl == "" {
		return fmt.Errorf("pathToIdl is empty")
	}
	if programName == "" {
		return fmt.Errorf("programName is empty")
	}
	if outputDir == "" {
		return fmt.Errorf("outputDir is empty")
	}
	if err := os.MkdirAll(outputDir, 0o777); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	slog.Info("Starting code generation",
		"outputDir", outputDir,
		"pathToIdl", pathToIdl,
	)
	parsedIdl, err := idl.ParseFromFilepath(pathToIdl)
	if err != nil {
		return fmt.Errorf("failed to parse IDL: %w", err)
	}
	if parsedIdl == nil {
		return fmt.Errorf("parsedIdl is nil")
	}
	if err := parsedIdl.Validate(); err != nil {
		return fmt.Errorf("invalid IDL: %w", err)
	}
	if parsedIdl.Address == nil || parsedIdl.Address.IsZero() {
		return fmt.Errorf("address is empty in idl file: %s", pathToIdl)
	}
	slog.Info("Using IDL address as program ID", "address", parsedIdl.Address.String())

	parsedIdl.Metadata.Name = bin.ToSnakeForSighash(parsedIdl.Metadata.Name)
	// check that the name is not a reserved keyword:
	if parsedIdl.Metadata.Name != "" {
		if tools.IsReservedKeyword(parsedIdl.Metadata.Name) {
			slog.Warn("The IDL metadata.name is a reserved Go keyword: adding a suffix to avoid conflicts.",
				"name", parsedIdl.Metadata.Name,
				"reservedKeyword", token.Lookup(parsedIdl.Metadata.Name).String(),
			)
			// Add a suffix to the name to avoid conflicts with Go reserved keywords:
			parsedIdl.Metadata.Name += "_program"
		}
		if !tools.IsValidIdent(parsedIdl.Metadata.Name) {
			// add a prefix to the name to avoid conflicts with Go reserved keywords:
			parsedIdl.Metadata.Name = "my_" + parsedIdl.Metadata.Name
		}
	}

	packageName, err := normalizeGoPackageName(programName)
	if err != nil {
		return err
	}
	if err := generator.ValidateIDLDerivedIdentifiers(parsedIdl); err != nil {
		return fmt.Errorf("IDL contains names that cannot be mapped to valid Go identifiers: %w", err)
	}

	options := generator.GeneratorOptions{
		OutputDir:   outputDir,
		Package:     packageName,
		ProgramName: programName,
		ProgramId:   parsedIdl.Address,
	}

	slog.Info("Parsed IDL successfully",
		"version", parsedIdl.Metadata.Version,
		"name", parsedIdl.Metadata.Name,
		"address", parsedIdl.Address,
		"programId", parsedIdl.Address.String(),
		"instructionsCount", len(parsedIdl.Instructions),
		"accountsCount", len(parsedIdl.Accounts),
		"eventsCount", len(parsedIdl.Events),
		"typesCount", len(parsedIdl.Types),
		"constantsCount", len(parsedIdl.Constants),
		"errorsCount", len(parsedIdl.Errors),
	)

	gen := generator.NewGenerator(parsedIdl, &options)
	generatedFiles, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("failed to generate: %w", err)
	}

	for _, file := range generatedFiles.Files {
		assetFilename := file.Name
		assetFilepath := path.Join(options.OutputDir, assetFilename)

		var buf bytes.Buffer
		if err := file.File.Render(&buf); err != nil {
			return fmt.Errorf("failed to render generated file %q: %w", assetFilename, err)
		}

		slog.Info("Writing file",
			"filepath", assetFilepath,
			"name", file.Name,
			"modPath", options.ModPath,
		)
		if err := os.WriteFile(assetFilepath, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("failed to write file %q: %w", assetFilepath, err)
		}
	}
	slog.Info("Generation completed successfully",
		"outputDir", options.OutputDir,
		"modPath", options.ModPath,
		"package", options.Package,
		"programName", options.ProgramName,
	)
	return nil
}

// normalizeGoPackageName maps a contract filename stem or program label to a valid Go package name.
func normalizeGoPackageName(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("contract/program name for Go package is empty")
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r == '-' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if !tools.IsValidIdent(out) {
		return "", fmt.Errorf("invalid Go package name after normalization (from contract/program name %q): %q is not a valid Go identifier; use only letters, digits, and underscores, and do not start with a digit", name, out)
	}
	if tools.IsReservedKeyword(out) {
		return "", fmt.Errorf("invalid Go package name: normalized name %q is a Go reserved keyword (from contract/program name %q)", out, name)
	}
	return out, nil
}
