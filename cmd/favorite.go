package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func newFavoriteCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "favorite",
		Short: "Manage global favorite paths",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(newFavoriteAddCommand())
	command.AddCommand(newFavoriteListCommand())
	command.AddCommand(newFavoriteRemoveCommand())
	return command
}

func newFavoriteAddCommand() *cobra.Command {
	var favoriteName string

	command := &cobra.Command{
		Use:     "add <path>",
		Short:   "Save a global favorite path",
		Args:    cobra.ExactArgs(1),
		Example: `wsx favorite add C:\src\repos --name WORK_REPOS`,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(favoriteName)
			if name == "" {
				return errors.New("favorite name cannot be empty")
			}

			path := strings.TrimSpace(args[0])
			if path == "" {
				return errors.New("favorite path cannot be empty")
			}

			resolvedPath, err := resolveFavoritePath(path)
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

			store, err := loadFavoriteStoreOrEmpty()
			if err != nil {
				return err
			}

			if err := store.Add(workspace.Favorite{
				Name:  name,
				Path:  resolvedPath,
				Added: time.Now().UTC(),
			}); err != nil {
				return err
			}

			if err := workspace.SaveFavoriteStore(store); err != nil {
				return err
			}

			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Added %q -> %s\n", name, resolvedPath)
			return writeErr
		},
	}

	command.Flags().StringVar(&favoriteName, "name", "", "Name to assign to the favorite")
	return command
}

func newFavoriteListCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List saved global favorites",
		Args:  cobra.NoArgs,
		Example: `wsx favorite list
wsx favorite list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := loadFavoriteStoreOrEmpty()
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeFavoriteJSON(cmd, store.Favorites)
			}

			return writeFavoriteTable(cmd, store.Favorites)
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output favorites as JSON")
	return command
}

func newFavoriteRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a saved favorite",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return errors.New("favorite name cannot be empty")
			}

			store, err := loadFavoriteStoreOrEmpty()
			if err != nil {
				return err
			}

			removed, ok := store.Remove(name)
			if !ok {
				return fmt.Errorf("favorite %q not found", name)
			}

			if err := workspace.SaveFavoriteStore(store); err != nil {
				return err
			}

			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", removed.Name)
			return writeErr
		},
	}
}

func loadFavoriteStoreOrEmpty() (workspace.FavoriteStore, error) {
	store, err := workspace.LoadFavoriteStore()
	if err == nil {
		return store, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return workspace.FavoriteStore{}, nil
	}
	return workspace.FavoriteStore{}, err
}

func resolveFavoritePath(input string) (string, error) {
	resolved := strings.TrimSpace(input)
	if !filepath.IsAbs(resolved) {
		absolute, err := filepath.Abs(resolved)
		if err != nil {
			return "", err
		}
		resolved = absolute
	}

	return filepath.Clean(resolved), nil
}

func writeFavoriteJSON(cmd *cobra.Command, favorites []workspace.Favorite) error {
	data, err := json.MarshalIndent(favorites, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeFavoriteTable(cmd *cobra.Command, favorites []workspace.Favorite) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, "NAME\tPATH\tADDED"); err != nil {
		return err
	}

	for _, favorite := range favorites {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", favorite.Name, favorite.Path, formatFavoriteAdded(favorite.Added)); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func formatFavoriteAdded(added time.Time) string {
	if added.IsZero() {
		return ""
	}

	return added.UTC().Format(time.RFC3339)
}

func sameFavoriteName(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}
