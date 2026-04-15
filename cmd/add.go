package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

const workspaceInstructionFilesStaleWarning = "Warning: workspace instruction files may be stale; run wsx agent-init\n"

func newAddCommand() *cobra.Command {
	var linkName string
	var favoriteName string

	command := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a linked repository to the current workspace",
		Args: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(favoriteName) != "" {
				if len(args) != 0 {
					return errors.New("favorite mode does not accept a path argument")
				}
				return nil
			}

			return cobra.ExactArgs(1)(cmd, args)
		},
		Example: `wsx add C:\src\repos\auth-service
wsx add C:\src\repos\payments-api --as payments
wsx add --favorite AUTH_SERVICE`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			inputPath := ""
			if strings.TrimSpace(favoriteName) != "" {
				store, err := loadFavoriteStoreOrEmpty()
				if err != nil {
					return err
				}

				favorite, ok := store.Get(favoriteName)
				if !ok {
					return fmt.Errorf("favorite %q not found", favoriteName)
				}

				inputPath = favorite.Path
			} else {
				inputPath = strings.TrimSpace(args[0])
			}

			resolvedPath, err := workspace.ResolveInputPath(inputPath)
			if err != nil {
				return err
			}

			info, err := os.Stat(resolvedPath)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("path is not a directory: %s", resolvedPath)
			}

			if err := rejectCircularReference(loaded.Root, resolvedPath); err != nil {
				return err
			}

			name := strings.TrimSpace(linkName)
			if name == "" {
				name = filepath.Base(resolvedPath)
			}
			if name == "" || name == "." {
				return errors.New("link name cannot be empty")
			}

			if err := rejectNameConflict(loaded, name); err != nil {
				return err
			}

			storedPath := resolvedPath
			linkPath := filepath.Join(loaded.Root, name)
			linkType, err := workspace.CreateLink(resolvedPath, linkPath)
			if err != nil {
				return err
			}

			loaded.Config.Refs = append(loaded.Config.Refs, workspace.Ref{
				Name:  name,
				Path:  storedPath,
				Added: time.Now().UTC(),
			})

			if err := workspace.SaveConfig(loaded.Root, loaded.Config); err != nil {
				_ = workspace.RemoveLink(linkPath)
				return err
			}

			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Added %q -> %s (%s)\n", name, storedPath, linkType)
			if writeErr != nil {
				return writeErr
			}

			warnIfWorkspaceInstructionFilesMayBeStale(cmd, loaded.Root)
			return nil
		},
	}

	command.Flags().StringVar(&linkName, "as", "", "Name to use for the workspace link")
	command.Flags().StringVar(&favoriteName, "favorite", "", "Favorite name to add instead of providing a path")
	return command
}

func rejectCircularReference(workspaceRoot, target string) error {
	root := filepath.Clean(workspaceRoot)
	target = filepath.Clean(target)

	if samePath(root, target) {
		return fmt.Errorf("circular reference: %s is the workspace root", target)
	}

	if _, ok := relativeIfWithin(root, target); ok {
		return fmt.Errorf("circular reference: %s is inside the workspace", target)
	}

	if _, ok := relativeIfWithin(target, root); ok {
		return fmt.Errorf("circular reference: %s contains the workspace", target)
	}

	return nil
}

func rejectNameConflict(loaded *workspace.LoadedConfig, name string) error {
	for _, ref := range loaded.Config.Refs {
		if samePath(ref.Name, name) {
			return fmt.Errorf("name conflict: ref %q already exists", name)
		}
	}

	linkPath := filepath.Join(loaded.Root, name)
	if _, err := os.Lstat(linkPath); err == nil {
		return fmt.Errorf("name conflict: %s already exists in the workspace", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func relativeIfWithin(base, target string) (string, bool) {
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return "", false
	}

	if relative == "." {
		return "", true
	}

	if relative == ".." {
		return "", false
	}

	prefix := ".." + string(os.PathSeparator)
	if strings.HasPrefix(relative, prefix) {
		return "", false
	}

	return relative, true
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func warnIfWorkspaceInstructionFilesMayBeStale(cmd *cobra.Command, root string) {
	if !workspaceInstructionFilesExist(root) {
		return
	}

	_, _ = fmt.Fprint(cmd.ErrOrStderr(), workspaceInstructionFilesStaleWarning)
}

func workspaceInstructionFilesExist(root string) bool {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	return false
}
