package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

func newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a linked repository from the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("ref name cannot be empty")
			}

			index := -1
			for i, ref := range loaded.Config.Refs {
				if samePath(ref.Name, name) {
					index = i
					name = ref.Name
					break
				}
			}
			if index == -1 {
				return fmt.Errorf("ref %q not found", name)
			}

			linkPath := filepath.Join(loaded.Root, name)
			if err := workspace.RemoveLink(linkPath); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			}

			loaded.Config.Refs = append(loaded.Config.Refs[:index], loaded.Config.Refs[index+1:]...)
			if err := workspace.SaveConfig(loaded.Root, loaded.Config); err != nil {
				return err
			}

			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", name)
			return writeErr
		},
	}
}
