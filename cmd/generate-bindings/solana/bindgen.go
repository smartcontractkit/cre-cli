package solana

import (
	"fmt"
	"go/token"
	"log/slog"
	"os"
	"path"

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
	options := generator.GeneratorOptions{
		OutputDir:   outputDir,
		Package:     programName,
		ProgramName: programName,
	}
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
	if parsedIdl.Address != nil && !parsedIdl.Address.IsZero() {
		// If the IDL has an address, use it as the program ID:
		slog.Info("Using IDL address as program ID", "address", parsedIdl.Address.String())
		options.ProgramId = parsedIdl.Address
	} else {
		return fmt.Errorf("address is empty in idl file: %s", pathToIdl)
	}
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
		{
			// Save assets:
			assetFilename := file.Name
			assetFilepath := path.Join(options.OutputDir, assetFilename)

			// Create file:
			goFile, err := os.Create(assetFilepath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			slog.Info("Writing file",
				"filepath", assetFilepath,
				"name", file.Name,
				"modPath", options.ModPath,
			)
			err = file.File.Render(goFile)
			if err != nil {
				goFile.Close()
				return fmt.Errorf("failed to render file: %w", err)
			}
			goFile.Close()
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
