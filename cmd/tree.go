package cmd

import (
	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

func newTreeCommand() *cobra.Command {
	depth := 2
	var showAll bool

	command := &cobra.Command{
		Use:   "tree",
		Short: "Show a clean directory tree for the current workspace",
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

			repos := make([]ai.TreeRepo, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				resolvedPath, err := resolveStatusPath(ref, env)
				if err != nil {
					return err
				}

				repos = append(repos, ai.TreeRepo{
					Name: ref.Name,
					Root: resolvedPath,
				})
			}

			output, err := ai.RenderWorkspaceTree(loaded.Config.Name, repos, ai.TreeOptions{
				MaxDepth:       depth,
				IncludeIgnored: showAll,
			})
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write([]byte(output))
			return err
		},
	}

	command.Flags().IntVar(&depth, "depth", 2, "Limit tree depth per repository (default 2, 0 means unlimited)")
	command.Flags().BoolVar(&showAll, "all", false, "Show ignored and default-excluded files")
	return command
}
