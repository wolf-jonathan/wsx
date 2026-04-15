package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

var Version = "dev"

func NewRootCommand() *cobra.Command {
	disableWindowsExplorerDelay()

	root := &cobra.Command{
		Use:           "wsx",
		Short:         "Manage Workspace X Windows-first AI workspaces",
		Long:          "Manage Workspace X Windows-first AI workspaces with the wsx CLI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newAddCommand())
	root.AddCommand(newAgentCommand())
	root.AddCommand(newDoctorCommand())
	root.AddCommand(newExecCommand())
	root.AddCommand(newFetchCommand())
	root.AddCommand(newFavoriteCommand())
	root.AddCommand(newGrepCommand())
	root.AddCommand(newInitCommand())
	root.AddCommand(newListCommand())
	root.AddCommand(newPromptCommand())
	root.AddCommand(newRemoveCommand())
	root.AddCommand(newSkillInstallCommand())
	root.AddCommand(newSkillUninstallCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newTreeCommand())

	return root
}

func Execute() error {
	return ExecuteCommand(NewRootCommand())
}

func ExecuteCommand(command *cobra.Command) error {
	executedCommand, err := command.ExecuteC()
	if err == nil {
		return nil
	}

	if shouldShowHelp(err) {
		helpCommand := executedCommand
		if helpCommand == nil {
			helpCommand = command
		}
		if err := helpCommand.Help(); err != nil {
			return err
		}
	}

	return err
}

func shouldShowHelp(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	for _, fragment := range []string{
		"unknown command",
		"unknown flag",
		"accepts ",
		"requires at most",
		"requires at least",
		"requires exactly",
		"argument",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}

	return false
}

func disableWindowsExplorerDelay() {
	cobra.MousetrapHelpText = ""
}
