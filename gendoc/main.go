package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/smartcontractkit/dev-platform/cmd"
)

func main() {

	log.Println("Generating docs...")
	// Create the output directory if it doesn't exist
	outputDir := filepath.Join("docs")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.Mkdir(outputDir, 0755)
		if err != nil {
			log.Fatal("Error creating docs dir: " + err.Error())
		}
	}

	// Generate Markdown documentation
	err := doc.GenMarkdownTree(cmd.RootCmd, outputDir)
	if err != nil {
		log.Fatal("Error generating documentation: " + err.Error())
	}
	log.Println("Documentation generated in " + outputDir)
}
