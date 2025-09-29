package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

// FindWorkflowDirectory finds the directory containing the workflow with the given name
func FindWorkflowDirectory(workflowName string) (string, error) {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return "", err
	}

	// Search all subdirectories for workflow.yaml files containing the workflow name
	var matchingDirs []string
	err = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || d.Name() != constants.DefaultWorkflowSettingsFileName {
			return nil
		}

		// Parse the workflow.yaml file
		workflowDir := filepath.Dir(path)
		if hasWorkflowName(path, workflowName) {
			matchingDirs = append(matchingDirs, workflowDir)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error walking project directories: %w", err)
	}

	if len(matchingDirs) == 0 {
		return "", fmt.Errorf("no workflow found with name '%s'", workflowName)
	}

	if len(matchingDirs) > 1 {
		return "", fmt.Errorf("more than one workflow found with the name '%s' in directories: %s", workflowName, strings.Join(matchingDirs, ", "))
	}

	return matchingDirs[0], nil
}

// findProjectRoot finds the project root by looking for project.yaml file
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		projectFile := filepath.Join(dir, constants.DefaultProjectSettingsFileName)
		if _, err := os.Stat(projectFile); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find %s in current directory or any parent directory", constants.DefaultProjectSettingsFileName)
}

// hasWorkflowName checks if the workflow.yaml file contains the target workflow name
func hasWorkflowName(workflowYamlPath, targetName string) bool {
	data, err := os.ReadFile(workflowYamlPath)
	if err != nil {
		return false
	}

	var yamlContent map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlContent); err != nil {
		return false
	}

	// Walk through the YAML structure to find workflow-name values
	for _, profileData := range yamlContent {
		if profileMap, ok := profileData.(map[string]interface{}); ok {
			if userWorkflow, exists := profileMap["user-workflow"]; exists {
				if userWorkflowMap, ok := userWorkflow.(map[string]interface{}); ok {
					if workflowNameValue, exists := userWorkflowMap["workflow-name"]; exists {
						if workflowName, ok := workflowNameValue.(string); ok && workflowName == targetName {
							return true
						}
					}
				}
			}
		}
	}

	return false
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
