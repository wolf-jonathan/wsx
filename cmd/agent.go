package cmd

import (
	"fmt"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

func newAgentCommand() *cobra.Command {
	var purpose string

	command := &cobra.Command{
		Use:   "agent-init",
		Short: "Generate workspace AI instruction files",
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
			if err := ai.WriteWorkspaceInstructionFiles(loaded.Root, content); err != nil {
				return err
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
