package cmd

import (
	"fmt"
	"os"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/spf13/cobra"
)

func newSkillInstallCommand() *cobra.Command {
	var scope string

	command := &cobra.Command{
		Use:   "skill-install",
		Short: "Install the bundled wsx SKILL.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			result, err := ai.InstallBundledSkill(cwd, scope)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed wsx skill to %s\n", result.Directory)
			return err
		},
	}

	command.Flags().StringVar(&scope, "scope", ai.SkillScopeLocal, "Install scope: local or global")
	return command
}
