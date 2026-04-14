package cmd

import (
	"fmt"
	"strings"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

func newAgentCommand() *cobra.Command {
	var purpose string

	command := &cobra.Command{
		Use:   "agent-init",
		Short: "Generate or refresh workspace AI instruction files",
		Long:  "Generate synchronized workspace AI instruction files. Existing CLAUDE.md and AGENTS.md files are overwritten with a warning.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			env, err := loadWorkspaceEnv(loaded.Root)
			if err != nil {
				return err
			}

			repos := make([]ai.InstructionRepo, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				resolvedPath, err := resolveStatusPath(ref, env)
				if err != nil {
					return err
				}

				repos = append(repos, ai.InstructionRepo{
					Name: ref.Name,
					Root: resolvedPath,
				})
			}

			instructions, err := ai.GenerateWorkspaceInstructions(loaded.Config.Name, purpose, repos)
			if err != nil {
				return err
			}

			content := ai.RenderWorkspaceInstructions(instructions)
			overwritten, err := ai.WriteWorkspaceInstructionFiles(loaded.Root, content)
			if err != nil {
				return err
			}

			if len(overwritten) > 0 {
				_, warnErr := fmt.Fprintf(
					cmd.ErrOrStderr(),
					"Warning: overwrote existing %s\n",
					strings.Join(overwritten, ", "),
				)
				if warnErr != nil {
					return warnErr
				}
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Wrote %s and %s\n",
				ai.WorkspaceClaudeFilePath,
				ai.WorkspaceAgentsFilePath,
			)
			return err
		},
	}

	command.Flags().StringVar(&purpose, "purpose", "", "Optional workspace purpose to include in generated instructions")
	return command
}
