package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

func newAddCommand() *cobra.Command {
	var linkName string

	command := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a linked repository to the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			env, err := loadWorkspaceEnv(loaded.Root)
			if err != nil {
				return err
			}

			inputPath := strings.TrimSpace(args[0])
			if inputPath == "" {
				return errors.New("path cannot be empty")
			}

			resolvedPath, err := resolveAddInputPath(inputPath, env)
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

			storedPath := chooseStoredPath(inputPath, resolvedPath, env)
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
			return writeErr
		},
	}

	command.Flags().StringVar(&linkName, "as", "", "Name to use for the workspace link")
	return command
}

func loadWorkspaceEnv(root string) (workspace.EnvVars, error) {
	env, err := workspace.LoadEnv(root)
	if err == nil {
		return env, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return workspace.EnvVars{}, nil
	}
	return nil, err
}

func resolveAddInputPath(inputPath string, env workspace.EnvVars) (string, error) {
	resolved := inputPath
	if strings.Contains(inputPath, "${") {
		var err error
		resolved, err = workspace.ResolvePath(inputPath, env)
		if err != nil {
			return "", err
		}
	}

	if !filepath.IsAbs(resolved) {
		absolute, err := filepath.Abs(resolved)
		if err != nil {
			return "", err
		}
		resolved = absolute
	}

	return filepath.Clean(resolved), nil
}

func chooseStoredPath(inputPath, absolutePath string, env workspace.EnvVars) string {
	if strings.Contains(inputPath, "${") {
		return normalizePortablePath(inputPath)
	}

	parameterized, ok := parameterizePath(absolutePath, env)
	if ok {
		return parameterized
	}

	return normalizePortablePath(absolutePath)
}

func parameterizePath(absolutePath string, env workspace.EnvVars) (string, bool) {
	cleanTarget := filepath.Clean(absolutePath)
	bestName := ""
	bestValue := ""

	for name, value := range env {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}

		cleanValue := filepath.Clean(value)
		relative, ok := relativeIfWithin(cleanValue, cleanTarget)
		if !ok {
			continue
		}

		candidate := fmt.Sprintf("${%s}", name)
		if relative != "" {
			candidate += "/" + normalizePortablePath(relative)
		}

		if len(cleanValue) > len(bestValue) {
			bestName = candidate
			bestValue = cleanValue
		}
	}

	if bestName == "" {
		return "", false
	}

	return bestName, true
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

func normalizePortablePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
