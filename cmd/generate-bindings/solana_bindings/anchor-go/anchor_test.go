package main_test

import (
	"fmt"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/gagliardetto/anchor-go/generator"
	"github.com/gagliardetto/anchor-go/idl"
	"github.com/gagliardetto/anchor-go/tools"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

const defaultProgramName = "myprogram"

func TestAnchorGo(t *testing.T) {

	var outputDir = "/Users/yashvardhan/cre-cli/cmd/generate-bindings/solana_bindings/testdata/my_anchor_project"
	var programName = "my_project"
	var modPath = ""
	var pathToIdl = "/Users/yashvardhan/cre-client-program/my-project/target/idl/my_project.json"
	var programIDOverride solana.PublicKey
	programIDOverride = solana.MustPublicKeyFromBase58("2GvhVcTPPkHbGduj6efNowFoWBQjE77Xab1uBKCYJvNN")

	modPath = path.Join("github.com", "gagliardetto", "anchor-go", "generated")
	slog.Info("Using default module path", "modPath", modPath)
	if err := os.MkdirAll(outputDir, 0o777); err != nil {
		panic(fmt.Errorf("Failed to create output directory: %w", err))
	}
	slog.Info("Starting code generation",
		"outputDir", outputDir,
		"modPath", modPath,
		"pathToIdl", pathToIdl,
		"programID", func() string {
			if programIDOverride.IsZero() {
				return "not provided"
			}
			return programIDOverride.String()
		}(),
	)

	options := generator.GeneratorOptions{
		OutputDir:   outputDir,
		Package:     programName,
		ProgramName: programName,
		ModPath:     modPath,
		SkipGoMod:   true,
	}
	if !programIDOverride.IsZero() {
		options.ProgramId = &programIDOverride
		slog.Info("Using provided program ID", "programID", programIDOverride.String())
	}
	parsedIdl, err := idl.ParseFromFilepath(pathToIdl)
	if err != nil {
		panic(err)
	}
	if parsedIdl == nil {
		panic("Parsed IDL is nil, please check the IDL file path and format.")
	}
	if err := parsedIdl.Validate(); err != nil {
		panic(fmt.Errorf("Invalid IDL: %w", err))
	}
	{
		{
			if parsedIdl.Address != nil && !parsedIdl.Address.IsZero() && options.ProgramId == nil {
				// If the IDL has an address, use it as the program ID:
				slog.Info("Using IDL address as program ID", "address", parsedIdl.Address.String())
				options.ProgramId = parsedIdl.Address
			}
		}
		parsedIdl.Metadata.Name = bin.ToSnakeForSighash(parsedIdl.Metadata.Name)
		{
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
			// if begins with
		}
		if programName == "" && parsedIdl.Metadata.Name != "" {
			panic("Please provide a package name using the -name flag, or ensure the IDL has a valid metadata.name field.")
		}
		if programName == defaultProgramName && parsedIdl.Metadata.Name != "" {
			cleanedName := bin.ToSnakeForSighash(parsedIdl.Metadata.Name)
			options.Package = cleanedName
			options.ProgramName = cleanedName
			slog.Info("Using IDL metadata.name as package name", "packageName", cleanedName)
		}

		slog.Info("Parsed IDL successfully",
			"version", parsedIdl.Metadata.Version,
			"name", parsedIdl.Metadata.Name,
			"address", parsedIdl.Address,
			"programId", func() string {
				if parsedIdl.Address.IsZero() {
					return "not provided"
				}
				return parsedIdl.Address.String()
			}(),
			"instructionsCount", len(parsedIdl.Instructions),
			"accountsCount", len(parsedIdl.Accounts),
			"eventsCount", len(parsedIdl.Events),
			"typesCount", len(parsedIdl.Types),
			"constantsCount", len(parsedIdl.Constants),
			"errorsCount", len(parsedIdl.Errors),
		)
	}
	gen := generator.NewGenerator(parsedIdl, &options)
	generatedFiles, err := gen.Generate()
	if err != nil {
		panic(err)
	}

	{
		for _, file := range generatedFiles.Files {
			{
				// Save assets:
				assetFilename := file.Name
				assetFilepath := path.Join(options.OutputDir, assetFilename)

				// Create file:
				goFile, err := os.Create(assetFilepath)
				if err != nil {
					panic(err)
				}
				defer goFile.Close()

				slog.Info("Writing file",
					"filepath", assetFilepath,
					"name", file.Name,
					"modPath", options.ModPath,
				)
				err = file.File.Render(goFile)
				if err != nil {
					panic(err)
				}
			}
		}
		// executeCmd(outputDir, "go", "mod", "tidy")
		// executeCmd(outputDir, "go", "fmt")
		// executeCmd(outputDir, "go", "build", "-o", "/dev/null") // Just to ensure everything compiles.
		slog.Info("Generation completed successfully",
			"outputDir", options.OutputDir,
			"modPath", options.ModPath,
			"package", options.Package,
			"programName", options.ProgramName,
		)
	}
}

func executeCmd(dir string, name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
