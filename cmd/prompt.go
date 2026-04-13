package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

var writePromptClipboard = copyPromptToClipboard

func newPromptCommand() *cobra.Command {
	var copyOutput bool

	command := &cobra.Command{
		Use:   "prompt",
		Short: "Generate an AI system prompt for the current workspace",
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

			treeRepos := make([]ai.TreeRepo, 0, len(loaded.Config.Refs))
			promptRepos := make([]ai.PromptRepo, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				resolvedPath, resolveErr := resolveStatusPath(ref, env)
				if resolveErr != nil {
					return resolveErr
				}

				detection, detectErr := ai.DetectRepo(resolvedPath)
				if detectErr != nil {
					return detectErr
				}

				absoluteRoot, absErr := filepath.Abs(resolvedPath)
				if absErr != nil {
					return absErr
				}
				displayRoot := filepath.ToSlash(absoluteRoot)

				treeRepos = append(treeRepos, ai.TreeRepo{
					Name: ref.Name,
					Root: resolvedPath,
				})
				promptRepos = append(promptRepos, ai.PromptRepo{
					Name:      ref.Name,
					Root:      displayRoot,
					Detection: detection,
				})
			}

			tree, err := ai.RenderWorkspaceTree(loaded.Config.Name, treeRepos, ai.TreeOptions{MaxDepth: 2})
			if err != nil {
				return err
			}

			output := ai.RenderWorkspacePrompt(ai.WorkspacePrompt{
				WorkspaceName: loaded.Config.Name,
				Repos:         promptRepos,
				Tree:          tree,
			})

			if copyOutput {
				if err := writePromptClipboard(output); err != nil {
					return err
				}
			}

			_, err = cmd.OutOrStdout().Write([]byte(output))
			return err
		},
	}

	command.Flags().BoolVar(&copyOutput, "copy", false, "Copy the generated prompt to the system clipboard")
	return command
}

func copyPromptToClipboard(content string) error {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		command = exec.Command("cmd", "/c", "clip")
	case "darwin":
		command = exec.Command("pbcopy")
	default:
		command = exec.Command("sh", "-c", "command -v wl-copy >/dev/null 2>&1 && wl-copy || command -v xclip >/dev/null 2>&1 && xclip -selection clipboard || xsel --clipboard --input")
	}

	command.Stdin = strings.NewReader(content)
	if output, err := command.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("copy prompt to clipboard: %s", message)
		}
		return fmt.Errorf("copy prompt to clipboard: %w", err)
	}

	return nil
}
