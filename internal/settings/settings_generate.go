package settings

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/context"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

//go:embed template/project.yaml.tpl
var ProjectSettingsTemplateContent string

//go:embed template/.env.tpl
var ProjectEnvironmentTemplateContent string

//go:embed template/.gitignore.tpl
var gitIgnoreTemplateContent string

//go:embed template/workflow.yaml.tpl
var workflowSettingsTemplateContent string

type ProjectEnv struct {
	FilePath       string
	GitHubAPIToken string
	EthPrivateKey  string
}

func GetDefaultReplacements() map[string]string {
	return map[string]string{
		"EthSepoliaChainName": constants.DefaultEthSepoliaChainName,
		"EthMainnetChainName": constants.DefaultEthMainnetChainName,

		"EthSepoliaRpcUrl": constants.DefaultEthSepoliaRpcUrl,
		"EthMainnetRpcUrl": constants.DefaultEthMainnetRpcUrl,
		"SethConfigPath":   constants.DefaultSethConfigPath,

		"ConfigPath":           "./config.json",
		"ConfigPathStaging":    "./config.staging.json",
		"ConfigPathProduction": "./config.production.json",
		"SecretsPath":          "",
	}
}

func GenerateFileFromTemplate(outputPath string, templateContent string, replacements map[string]string) error {
	var replacerArgs []string
	for key, value := range replacements {
		replacerArgs = append(replacerArgs, "{{"+key+"}}", value)
	}
	replacer := strings.NewReplacer(replacerArgs...)

	finalContent := replacer.Replace(string(templateContent))

	if err := os.WriteFile(outputPath, []byte(finalContent), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func GenerateProjectEnvFile(workingDirectory string) (string, error) {
	outputPath, err := filepath.Abs(path.Join(workingDirectory, constants.DefaultEnvFileName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for writing file: %w", err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		shouldContinue, err := ui.Confirm(
			fmt.Sprintf("A project environment file already exists at %s. Continuing will overwrite this file.", outputPath),
			ui.WithDescription("Do you want to proceed?"),
		)
		if err != nil {
			return "", fmt.Errorf("failed to prompt for file overwrite confirmation: %w", err)
		}
		if !shouldContinue {
			return "", fmt.Errorf("operation aborted by user")
		}
	}

	replacements := map[string]string{
		"GithubApiToken": "your-github-token",
		"EthPrivateKey":  "your-eth-private-key",
	}

	if err := GenerateFileFromTemplate(outputPath, ProjectEnvironmentTemplateContent, replacements); err != nil {
		return "", fmt.Errorf("failed to generate .env file: %w", err)
	}

	_, err = GenerateGitIgnoreFile(workingDirectory)
	if err != nil {
		return "", fmt.Errorf("failed to generate .gitignore file: %w", err)
	}

	return outputPath, nil
}

func GenerateProjectSettingsFile(workingDirectory string) (string, bool, error) {
	replacements := GetDefaultReplacements()

	outputPath, err := filepath.Abs(path.Join(workingDirectory, constants.DefaultProjectSettingsFileName))
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve absolute path for writing file: %w", err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		shouldContinue, err := ui.Confirm(
			fmt.Sprintf("A project settings file already exists at %s. Continuing will overwrite this file.", outputPath),
			ui.WithDescription("Do you want to proceed?"),
		)
		if err != nil {
			return "", false, fmt.Errorf("failed to prompt for file overwrite confirmation: %w", err)
		}
		if !shouldContinue {
			return "", true, nil
		}
	}

	if err := GenerateFileFromTemplate(outputPath, ProjectSettingsTemplateContent, replacements); err != nil {
		return "", false, fmt.Errorf("failed to generate project settings file: %w", err)
	}

	return outputPath, true, nil
}

func FindOrCreateProjectSettings(startDir string, replacements map[string]string) error {
	_, isFound, err := context.FindProjectSettingsPath(startDir)
	if err != nil {
		return err
	}

	if !isFound {
		projectSettingsOutputPath, err := filepath.Abs(path.Join(startDir, constants.DefaultProjectSettingsFileName))
		if err != nil {
			return err
		}
		return GenerateFileFromTemplate(projectSettingsOutputPath, ProjectSettingsTemplateContent, replacements)
	}
	return nil
}

func GenerateWorkflowSettingsFile(workingDirectory string, workflowName string, workflowPath string) (string, error) {
	// Use default replacements.
	replacements := GetDefaultReplacements()
	replacements["WorkflowName"] = workflowName
	replacements["WorkflowPath"] = workflowPath

	// Resolve the absolute output path for the workflow settings file.
	outputPath, err := filepath.Abs(path.Join(workingDirectory, constants.DefaultWorkflowSettingsFileName))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for writing file: %w", err)
	}

	// Generate the workflow settings file.
	if err := GenerateFileFromTemplate(outputPath, workflowSettingsTemplateContent, replacements); err != nil {
		return "", fmt.Errorf("failed to generate workflow settings file: %w", err)
	}

	return outputPath, nil
}

func GenerateGitIgnoreFile(workingDirectory string) (string, error) {
	gitIgnorePath := filepath.Join(workingDirectory, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); err == nil {
		outputPath, err := filepath.Abs(gitIgnorePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return outputPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	outputPath, err := filepath.Abs(gitIgnorePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for writing file: %w", err)
	}
	replacements := map[string]string{}
	if err := GenerateFileFromTemplate(outputPath, gitIgnoreTemplateContent, replacements); err != nil {
		return "", fmt.Errorf("failed to generate settings file: %w", err)
	}
	return outputPath, nil
}
