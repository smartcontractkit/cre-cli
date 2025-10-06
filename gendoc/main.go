package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/smartcontractkit/cre-cli/cmd"
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

	customizeUsageStrings(cmd.RootCmd)

	// Generate Markdown documentation
	err := doc.GenMarkdownTree(cmd.RootCmd, outputDir)
	if err != nil {
		log.Fatal("Error generating documentation: " + err.Error())
	}

	log.Println("Documentation generated in " + outputDir)
}

func customizeUsageStrings(rootCmd *cobra.Command) {
	var processCommand func(*cobra.Command)
	processCommand = func(c *cobra.Command) {
		if !strings.Contains(c.Use, "[") {
			// Disable automatic [flags] addition and add our own
			c.DisableFlagsInUseLine = true
			c.Use = c.Use + " [optional flags]"
		}

		// Process all subcommands recursively
		for _, subCmd := range c.Commands() {
			processCommand(subCmd)
		}
	}

	// Process the root command and all its subcommands
	processCommand(rootCmd)
}
