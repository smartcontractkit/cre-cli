package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/account"
	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/cmd/creinit"
	generatebindings "github.com/smartcontractkit/cre-cli/cmd/generate-bindings"
	"github.com/smartcontractkit/cre-cli/cmd/login"
	"github.com/smartcontractkit/cre-cli/cmd/logout"
	"github.com/smartcontractkit/cre-cli/cmd/secrets"
	"github.com/smartcontractkit/cre-cli/cmd/version"
	"github.com/smartcontractkit/cre-cli/cmd/whoami"
	"github.com/smartcontractkit/cre-cli/cmd/workflow"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	crecontext "github.com/smartcontractkit/cre-cli/internal/context"
	"github.com/smartcontractkit/cre-cli/internal/logger"
	creruntime "github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/telemetry"
	"github.com/smartcontractkit/cre-cli/internal/update"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = newRootCommand()

var runtimeContextForTelemetry *creruntime.Context

var executingCommand *cobra.Command

const telemetryPayloadEnvVar = "_CRE_TELEMETRY_PAYLOAD"

func Execute() {
	// Check if this process was spawned *only* to send telemetry
	if payload := os.Getenv(telemetryPayloadEnvVar); payload != "" {
		runDetachedTelemetry(payload)
		return // Exit after sending
	}

	// If not, run the normal CLI
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// This code runs in the new, detached process.
func runDetachedTelemetry(payloadBase64 string) {
	telemetry.DebugLog("DETACHED: Process started.")

	telemetry.DebugLog("DETACHED: Received raw base64 payload: %s", payloadBase64)

	// 1. Decode the event
	eventJSON, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		telemetry.DebugLog("DETACHED: failed to decode base64: %v", err)
		return
	}

	telemetry.DebugLog("DETACHED: Decoded JSON payload: %s", string(eventJSON))

	var event telemetry.UserEventInput
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		telemetry.DebugLog("DETACHED: failed to unmarshal json: %v", err)
		return
	}

	telemetry.DebugLog("DETACHED: Unmarshaled event struct: %+v", event)

	// 2. Create a minimal runtime context
	logStruct := createLogger().Level(zerolog.Disabled)
	log := &logStruct
	v := createViper()

	minimalRuntimeCtx := creruntime.NewContext(log, v)
	if err := minimalRuntimeCtx.AttachCredentials(); err != nil {
		telemetry.DebugLog("DETACHED: failed to attach credentials: %v", err)
		return
	}
	if err := minimalRuntimeCtx.AttachEnvironmentSet(); err != nil {
		telemetry.DebugLog("DETACHED: failed to attach env set: %v", err)
		return
	}

	// 3. Send the event
	telemetry.DebugLog("DETACHED: All setup complete. Sending event for command: %s", event.Command.Action)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	telemetry.SendEvent(ctx, event,
		minimalRuntimeCtx.Credentials,
		minimalRuntimeCtx.EnvironmentSet,
		minimalRuntimeCtx.Logger)
}

func newRootCommand() *cobra.Command {
	rootLogger := createLogger()
	rootViper := createViper()
	runtimeContext := creruntime.NewContext(rootLogger, rootViper)

	runtimeContextForTelemetry = runtimeContext

	// By defining a Run func, we force PersistentPreRunE to execute
	// even when 'cre', 'workflow', etc is called with no subcommand
	// this enables to check for update and display if needed
	helpRunE := func(cmd *cobra.Command, args []string) error {
		err := cmd.Help()
		if err != nil {
			return fmt.Errorf("fail to show help: %w", err)
		}
		return nil
	}

	rootCmd := &cobra.Command{
		Use:               "cre",
		Short:             "CRE CLI tool",
		Long:              `A command line tool for building, testing and managing Chainlink Runtime Environment (CRE) workflows.`,
		DisableAutoGenTag: true,
		RunE:              helpRunE,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			executingCommand = cmd

			log := runtimeContext.Logger
			v := runtimeContext.Viper

			if err := v.BindPFlags(cmd.Flags()); err != nil {
				return fmt.Errorf("failed to bind flags: %w", err)
			}

			if verbose := v.GetBool(settings.Flags.Verbose.Name); verbose {
				newLogger := log.Level(zerolog.DebugLevel)
				if _, found := os.LookupEnv("SETH_LOG_LEVEL"); !found {
					os.Setenv("SETH_LOG_LEVEL", "debug")
				}
				runtimeContext.Logger = &newLogger
				runtimeContext.ClientFactory = client.NewFactory(&newLogger, v)
			}

			if isLoadEnvAndSettings(cmd) {
				projectRootFlag := runtimeContext.Viper.GetString(settings.Flags.ProjectRoot.Name)
				if err := crecontext.SetExecutionContext(cmd, args, projectRootFlag, rootLogger); err != nil {
					return err
				}

				err := runtimeContext.AttachSettings(cmd)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
			}

			if isLoadCredentials(cmd) {
				err := runtimeContext.AttachCredentials()
				if err != nil {
					return fmt.Errorf("failed to attach credentials: %w", err)
				}
			}

			err := runtimeContext.AttachEnvironmentSet()
			if err != nil {
				return fmt.Errorf("failed to load environment details: %w", err)
			}

			return nil
		},

		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Check for updates
			if cmd.Name() != "bash" && cmd.Name() != "zsh" && cmd.Name() != "fish" && cmd.Name() != "powershell" && cmd.Name() != "help" {
				update.CheckForUpdates(version.Version, runtimeContext.Logger)
			}

			// 1. Check if command should be excluded
			if telemetry.ShouldExcludeCommand(cmd) {
				return
			}

			telemetry.DebugLog("MAIN: Starting telemetry for command: %s", cmd.Name())

			// 2. Build the event
			event := telemetry.BuildUserEvent(cmd, 0)

			// 3. Serialize event to JSON, then Base64
			eventJSON, err := json.Marshal(event)
			if err != nil {
				telemetry.DebugLog("MAIN: failed to marshal telemetry event: %v", err)
				return // Fail silently
			}
			eventBase64 := base64.StdEncoding.EncodeToString(eventJSON)

			// 4. Get path to current executable
			exe, err := os.Executable()
			if err != nil {
				telemetry.DebugLog("MAIN: failed to find executable path: %v", err)
				return // Fail silently
			}

			// 5. Prepare the detached command
			detachedCmd := exec.Command(exe)

			// Pass payload via environment variable
			detachedCmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", telemetryPayloadEnvVar, eventBase64))

			// Detach from parent process to run independently
			if runtime.GOOS == "windows" {
				detachedCmd.SysProcAttr = &syscall.SysProcAttr{
					// CreationFlags: 0x08000000, // CREATE_NO_WINDOW
				}
			} else {
				detachedCmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid: true,
				}
			}

			// Redirect detached process output to log file if debugging
			detachedCmd.Stdin = nil
			if telemetry.IsTelemetryDebugEnabled() {
				logPath := telemetry.GetLogfilePath()
				logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					detachedCmd.Stdout = logFile
					detachedCmd.Stderr = logFile
				} else {
					telemetry.DebugLog("MAIN: Could not open log file for detached process: %v", err)
					detachedCmd.Stdout = nil
					detachedCmd.Stderr = nil
				}
			} else {
				detachedCmd.Stdout = nil
				detachedCmd.Stderr = nil
			}

			// 6. Start the command and *do not* wait for it
			err = detachedCmd.Start()
			if err != nil {
				telemetry.DebugLog("MAIN: failed to start detached telemetry process: %v", err)
				return
			}

			// Release the process so it doesn't become a zombie
			if detachedCmd.Process != nil {
				telemetry.DebugLog("MAIN: Detached process started with PID: %d", detachedCmd.Process.Pid)
				detachedCmd.Process.Release()
			}
		},
	}

	cobra.AddTemplateFunc("wrappedFlagUsages", func(fs *pflag.FlagSet) string {
		// 100 = wrap width
		return strings.TrimRight(fs.FlagUsagesWrapped(100), "\n")
	})

	cobra.AddTemplateFunc("hasUngrouped", func(c *cobra.Command) bool {
		for _, cmd := range c.Commands() {
			if cmd.IsAvailableCommand() && !cmd.Hidden && cmd.GroupID == "" {
				return true
			}
		}
		return false
	})

	rootCmd.SetHelpTemplate(`
{{- with (or .Long .Short)}}{{.}}{{end}}

Usage:
{{- if .Runnable}}
  {{.UseLine}}
{{- else if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]
{{- end}}

{{- /* ============================================ */}}
{{- /* Available Commands Section                 */}}
{{- /* ============================================ */}}
{{- if .HasAvailableSubCommands}}

Available Commands:
  {{- $groupsUsed := false -}}
  {{- $firstGroup := true -}}

  {{- range $grp := .Groups}}
    {{- $has := false -}}
    {{- range $.Commands}}
      {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID $grp.ID))}}
        {{- $has = true}}
      {{- end}}
    {{- end}}
    
    {{- if $has}}
      {{- $groupsUsed = true -}}
      {{- if $firstGroup}}{{- $firstGroup = false -}}{{else}}

{{- end}}

  {{printf "%s:" $grp.Title}}
      {{- range $.Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID $grp.ID))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- end}}

  {{- if $groupsUsed }}
    {{- /* Groups are in use; show ungrouped as "Other" if any */}}
    {{- if hasUngrouped .}}

  Other:
      {{- range .Commands}}
        {{- if (and (not .Hidden) (.IsAvailableCommand) (eq .GroupID ""))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
        {{- end}}
      {{- end}}
    {{- end}}
  {{- else }}
    {{- /* No groups at this level; show a flat list with no "Other" header */}}
    {{- range .Commands}}
      {{- if (and (not .Hidden) (.IsAvailableCommand))}}
    {{rpad .Name .NamePadding}}  {{.Short}}
      {{- end}}
    {{- end}}
  {{- end }}
{{- end }}

{{- if .HasExample}}

Examples:
{{.Example}}
{{- end }}

{{- $local := (.LocalFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $local }}

Flags:
{{$local}}
{{- end }}

{{- $inherited := (.InheritedFlags.FlagUsagesWrapped 100 | trimTrailingWhitespaces) -}}
{{- if $inherited }}

Global Flags:
{{$inherited}}
{{- end }}

{{- if .HasAvailableSubCommands }}

Use "{{.CommandPath}} [command] --help" for more information about a command.
{{- end }}

💡 Tip: New here? Run:
  $ cre login
    to login into your cre account, then:
  $ cre init
    to create your first cre project.

📘 Need more help?
  Visit https://docs.chain.link/cre
`)

	// Definition of global flags:
	// env file flag is present for every subcommand
	rootCmd.PersistentFlags().StringP(
		settings.Flags.CliEnvFile.Name,
		settings.Flags.CliEnvFile.Short,
		constants.DefaultEnvFileName,
		fmt.Sprintf("Path to %s file which contains sensitive info", constants.DefaultEnvFileName),
	)

	// project root path flag is present for every subcommand
	rootCmd.PersistentFlags().StringP(
		settings.Flags.ProjectRoot.Name,
		settings.Flags.ProjectRoot.Short,
		"",
		"Path to the project root",
	)

	// verbose flag is present in every subcommand
	rootCmd.PersistentFlags().BoolP(
		settings.Flags.Verbose.Name,
		settings.Flags.Verbose.Short,
		false,
		"Run command in VERBOSE mode",
	)

	// target settings is present in every subcommand
	rootCmd.PersistentFlags().StringP(
		settings.Flags.Target.Name,
		settings.Flags.Target.Short,
		"",
		"Use target settings from YAML config",
	)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	secretsCmd := secrets.New(runtimeContext)
	workflowCmd := workflow.New(runtimeContext)
	versionCmd := version.New(runtimeContext)
	loginCmd := login.New(runtimeContext)
	logoutCmd := logout.New(runtimeContext)
	initCmd := creinit.New(runtimeContext)
	genBindingsCmd := generatebindings.New(runtimeContext)
	accountCmd := account.New(runtimeContext)
	whoamiCmd := whoami.New(runtimeContext)

	secretsCmd.RunE = helpRunE
	workflowCmd.RunE = helpRunE
	accountCmd.RunE = helpRunE

	// Define groups (order controls display order)
	rootCmd.AddGroup(&cobra.Group{ID: "getting-started", Title: "Getting Started"})
	rootCmd.AddGroup(&cobra.Group{ID: "account", Title: "Account"})
	rootCmd.AddGroup(&cobra.Group{ID: "workflow", Title: "Workflow"})
	rootCmd.AddGroup(&cobra.Group{ID: "secret", Title: "Secret"})

	initCmd.GroupID = "getting-started"

	loginCmd.GroupID = "account"
	logoutCmd.GroupID = "account"
	accountCmd.GroupID = "account"
	whoamiCmd.GroupID = "account"

	secretsCmd.GroupID = "secret"
	workflowCmd.GroupID = "workflow"

	rootCmd.AddCommand(
		initCmd,
		versionCmd,
		loginCmd,
		logoutCmd,
		accountCmd,
		whoamiCmd,
		secretsCmd,
		workflowCmd,
		genBindingsCmd,
	)

	return rootCmd
}

func isLoadEnvAndSettings(cmd *cobra.Command) bool {
	// It is not expected to have the .env and the settings file when running the following commands
	var excludedCommands = map[string]struct{}{
		"version":           {},
		"login":             {},
		"logout":            {},
		"whoami":            {},
		"list-key":          {},
		"init":              {},
		"generate-bindings": {},
		"bash":              {},
		"fish":              {},
		"powershell":        {},
		"zsh":               {},
		"help":              {},
		"cre":               {},
		"account":           {},
		"secrets":           {},
		"workflow":          {},
	}

	_, exists := excludedCommands[cmd.Name()]
	return !exists
}

func isLoadCredentials(cmd *cobra.Command) bool {
	// It is not expected to have the credentials loaded when running the following commands
	var excludedCommands = map[string]struct{}{
		"version":           {},
		"login":             {},
		"bash":              {},
		"fish":              {},
		"powershell":        {},
		"zsh":               {},
		"help":              {},
		"generate-bindings": {},
		"cre":               {},
		"account":           {},
		"secrets":           {},
		"workflow":          {},
	}

	_, exists := excludedCommands[cmd.Name()]
	return !exists
}

func createLogger() *zerolog.Logger {
	// Set default Seth log level if not set
	if _, found := os.LookupEnv("SETH_LOG_LEVEL"); !found {
		os.Setenv("SETH_LOG_LEVEL", constants.DefaultSethLogLevel)
	}

	return logger.NewConsoleLogger()
}

func createViper() *viper.Viper {
	return viper.New() //nolint:forbidigo
}
