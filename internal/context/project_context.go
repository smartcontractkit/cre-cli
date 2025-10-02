package context

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/transformation"
)

// SetExecutionContext sets the appropriate execution context for commands
// It first sets the project context, then if it's a workflow command with exactly one argument,
// it changes to the specific workflow directory
func SetExecutionContext(cmd *cobra.Command, args []string, projectRootFlag string, logger *zerolog.Logger) error {
	// Check if project-root flag is set
	var projectPath string
	if projectRootFlag != "" {
		// Resolve the path (handles both relative and absolute paths)
		resolvedPath, err := filepath.Abs(projectRootFlag)
		if err != nil {
			return fmt.Errorf("failed to resolve project root path '%s': %w", projectRootFlag, err)
		}

		// Check if path exists
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			return fmt.Errorf("project root path does not exist: %s", resolvedPath)
		} else if err != nil {
			return fmt.Errorf("failed to check project root path '%s': %w", resolvedPath, err)
		}

		projectPath = resolvedPath
	}

	// First, set the project context (change to project root)
	if err := SetProjectContext(projectPath); err != nil {
		return fmt.Errorf("failed to set project context: %w", err)
	}

	// Then, if it's a workflow command with exactly one argument, change to the workflow directory
	if IsWorkflowCommand(cmd) && len(args) == 1 {
		workflowDir, err := transformation.ResolveWorkflowPath(args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve workflow directory path '%s': %w", args[0], err)
		}
		if err := os.Chdir(workflowDir); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", workflowDir, err)
		}
	}

	return nil
}

// SetProjectContext sets the current working directory to the project root
// If projectPath is provided and not empty, it uses that as the project root
// Otherwise, it finds the project root using FindProjectSettingsPath
func SetProjectContext(projectPath string) error {
	var projectRoot string

	if projectPath != "" {
		// Use the provided project path directly
		projectRoot = projectPath
	} else {
		// Get the current working directory as the starting point
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		// Find the project settings file
		projectSettingsPath, found, err := FindProjectSettingsPath(cwd)
		if err != nil {
			return fmt.Errorf("failed to find project settings: %w", err)
		}

		if !found {
			return fmt.Errorf("no project settings file found in current directory or parent directories")
		}

		// Get the directory containing the project settings file (this is the project root)
		projectRoot = filepath.Dir(projectSettingsPath)
	}

	// Set the project root as the current working directory
	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("failed to change directory to project root %s: %w", projectRoot, err)
	}

	return nil
}

func FindProjectSettingsPath(startDir string) (string, bool, error) {
	var err error

	if startDir == "" {
		return "", false, fmt.Errorf("starting directory cannot be empty")
	}

	cwd := startDir

	for {
		filePath := filepath.Join(cwd, constants.DefaultProjectSettingsFileName)
		if _, err = os.Stat(filePath); err == nil {
			return filePath, true, nil // File exists, return the path and true
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("error checking project settings: %w", err)
		}

		parentDir := filepath.Dir(cwd)
		if parentDir == cwd {
			break // Stop if we can't go up further
		}
		cwd = parentDir
	}

	return "", false, nil
}

// IsWorkflowCommand checks if the command is a workflow command
func IsWorkflowCommand(cmd *cobra.Command) bool {
	// Check if the current command or any parent command is "workflow"
	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == "workflow" {
			return true
		}
	}
	return false
}
