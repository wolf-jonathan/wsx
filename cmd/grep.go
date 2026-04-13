package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

type grepCommandError struct {
	message string
}

func (e *grepCommandError) Error() string {
	return e.message
}

func newGrepCommand() *cobra.Command {
	var include string
	var exclude string
	var jsonOutput bool
	var contextLines int

	command := &cobra.Command{
		Use:   "grep <pattern>",
		Short: "Search for a pattern across linked repositories",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := strings.TrimSpace(args[0])
			if pattern == "" {
				return errors.New("pattern cannot be empty")
			}
			if contextLines < 0 {
				return errors.New("context cannot be negative")
			}

			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			env, err := loadWorkspaceEnv(loaded.Root)
			if err != nil {
				return err
			}

			repos := make([]ai.GrepRepo, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				resolvedPath, err := resolveStatusPath(ref, env)
				if err != nil {
					return err
				}

				repos = append(repos, ai.GrepRepo{
					Name: ref.Name,
					Root: resolvedPath,
				})
			}

			matches, err := ai.GrepWorkspace(repos, pattern, ai.GrepOptions{
				IncludeGlobs: parseCommaPatterns(include),
				ExcludeGlobs: parseCommaPatterns(exclude),
				ContextLines: contextLines,
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				if err := writeGrepJSON(cmd, matches); err != nil {
					return err
				}
			} else {
				if err := writeGrepText(cmd, matches); err != nil {
					return err
				}
			}

			if len(matches) == 0 {
				return &grepCommandError{message: "no matches found"}
			}

			return nil
		},
	}

	command.Flags().StringVar(&include, "include", "", "Comma-separated glob patterns to include")
	command.Flags().StringVar(&exclude, "exclude", "", "Comma-separated glob patterns to exclude")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output grep matches as JSON")
	command.Flags().IntVar(&contextLines, "context", 0, "Show N lines of context around each match")
	return command
}

func parseCommaPatterns(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		patterns = append(patterns, filepathToSlash(part))
	}

	if len(patterns) == 0 {
		return nil
	}

	return patterns
}

func filepathToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func writeGrepJSON(cmd *cobra.Command, matches []ai.GrepMatch) error {
	data, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeGrepText(cmd *cobra.Command, matches []ai.GrepMatch) error {
	for _, match := range matches {
		for index, line := range match.Before {
			lineNumber := match.Line - len(match.Before) + index
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s]  %s:%d-  %s\n", match.Repo, match.File, lineNumber, line); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s]  %s:%d:  %s\n", match.Repo, match.File, match.Line, match.Match); err != nil {
			return err
		}

		for index, line := range match.After {
			lineNumber := match.Line + index + 1
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s]  %s:%d+  %s\n", match.Repo, match.File, lineNumber, line); err != nil {
				return err
			}
		}
	}

	return nil
}
