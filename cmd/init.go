package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

const configVersion = "2"

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a Workspace X workspace in the current directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			name := filepath.Base(cwd)
			if len(args) == 1 {
				name = strings.TrimSpace(args[0])
			}
			if name == "" {
				return errors.New("workspace name cannot be empty")
			}

			configPath := filepath.Join(cwd, workspace.ConfigFileName)
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("workspace already initialized: %s exists", workspace.ConfigFileName)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			cfg := workspace.Config{
				Version: configVersion,
				Name:    name,
				Created: time.Now().UTC(),
				Refs:    []workspace.Ref{},
			}

			if err := workspace.SaveConfig(cwd, cfg); err != nil {
				return err
			}

			if err := ensureGitignoreEntry(cwd, workspace.ConfigFileName); err != nil {
				return err
			}

			_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Initialized workspace %q in %s\n", name, cwd)
			return writeErr
		},
	}
}

func ensureGitignoreEntry(root, entry string) error {
	path := filepath.Join(root, ".gitignore")
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	lines := []string{}
	if len(content) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
		for _, line := range lines {
			if line == entry {
				return nil
			}
		}
	}

	var builder strings.Builder
	if len(content) > 0 {
		trimmed := strings.TrimRight(string(content), "\r\n")
		builder.WriteString(trimmed)
		builder.WriteByte('\n')
	}
	builder.WriteString(entry)
	builder.WriteByte('\n')

	return os.WriteFile(path, []byte(builder.String()), 0o644)
}
