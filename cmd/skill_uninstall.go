package cmd

import (
	"fmt"
	"os"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/spf13/cobra"
)

func newSkillUninstallCommand() *cobra.Command {
	var scope string

	command := &cobra.Command{
		Use:   "skill-uninstall",
		Short: "Remove an installed wsx skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			result, err := ai.UninstallBundledSkill(cwd, scope)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed wsx skill from %s\n", result.Directory)
			return err
		},
	}

	command.Flags().StringVar(&scope, "scope", ai.SkillScopeLocal, "Install scope: local or global")
	return command
}
