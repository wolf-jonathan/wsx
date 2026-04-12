package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "wsx",
		Short:         "Manage Windows-first AI workspaces",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func Execute() error {
	return NewRootCommand().Execute()
}
