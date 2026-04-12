package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "wsx",
		Short: "Manage Windows-first AI workspaces",
		Long: `Manage Windows-first AI workspaces.

Currently supported commands:
  add     Add a linked repository to the current workspace
  fetch   Run git fetch across linked repositories
  init    Initialize a workspace in the current directory
  list    List linked repositories in the current workspace
  remove  Remove a linked repository from the current workspace
  status  Run git status across linked repositories

Only implemented commands are shown below.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newAddCommand())
	root.AddCommand(newFetchCommand())
	root.AddCommand(newInitCommand())
	root.AddCommand(newListCommand())
	root.AddCommand(newRemoveCommand())
	root.AddCommand(newStatusCommand())

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
