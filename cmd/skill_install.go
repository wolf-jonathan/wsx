package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/internal/ai"
)

func newSkillInstallCommand() *cobra.Command {
	var scope string

	command := &cobra.Command{
		Use:   "skill-install",
		Short: "Install or refresh the bundled wsx SKILL.md",
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

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Installed wsx skill to %s\n", result.Directory); err != nil {
				return err
			}
			if result.ClaudeDirectory != "" {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Linked Claude skill to %s (%s)\n", result.ClaudeDirectory, result.ClaudeLinkType)
				return err
			}
			return nil
		},
	}

	command.Flags().StringVar(&scope, "scope", ai.SkillScopeLocal, "Install scope: local or global (global also links into ~/.claude/skills)")
	return command
}
