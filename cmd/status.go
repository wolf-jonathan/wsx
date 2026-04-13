package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	wsxgit "github.com/wolf-jonathan/workspace-x/internal/git"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
	"github.com/spf13/cobra"
)

const statusSummaryClean = "clean"

type gitStatusClient interface {
	Status(path string) (wsxgit.CommandResult, error)
}

var newStatusGitClient = func() gitStatusClient {
	return wsxgit.NewClient(nil)
}

type statusItem struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	ResolvedPath string `json:"resolved_path"`
	Summary      string `json:"summary"`
	Clean        bool   `json:"clean"`
	Error        string `json:"error,omitempty"`
	ExitCode     int    `json:"exit_code"`
}

type statusCommandError struct {
	message string
}

func (e *statusCommandError) Error() string {
	return e.message
}

func newStatusCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "status",
		Short: "Run git status across linked repositories",
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

			client := newStatusGitClient()
			items := make([]statusItem, 0, len(loaded.Config.Refs))
			hasIssues := false

			for _, ref := range loaded.Config.Refs {
				item := statusItem{
					Name:  ref.Name,
					Path:  ref.Path,
					Clean: false,
				}

				resolvedPath, resolveErr := resolveStatusPath(ref, env)
				if resolveErr != nil {
					item.Error = resolveErr.Error()
					item.Summary = "error: " + resolveErr.Error()
					items = append(items, item)
					hasIssues = true
					continue
				}

				item.ResolvedPath = resolvedPath

				result, statusErr := client.Status(resolvedPath)
				item.ExitCode = result.ExitCode
				if statusErr != nil {
					item.Error = statusErr.Error()
					if strings.TrimSpace(result.Stderr) != "" {
						item.Summary = "error: " + strings.TrimSpace(result.Stderr)
					} else {
						item.Summary = "error: " + statusErr.Error()
					}
					items = append(items, item)
					hasIssues = true
					continue
				}

				item.Summary, item.Clean = summarizeGitStatus(result.Stdout)
				if !item.Clean {
					hasIssues = true
				}

				items = append(items, item)
			}

			if jsonOutput {
				if err := writeStatusJSON(cmd, items); err != nil {
					return err
				}
			} else {
				if err := writeStatusTable(cmd, items); err != nil {
					return err
				}
			}

			if hasIssues {
				return &statusCommandError{message: "one or more repositories are dirty or unavailable"}
			}

			return nil
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output repository status as JSON")
	return command
}

func summarizeGitStatus(stdout string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n")
	changed := 0
	untracked := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "##") {
			continue
		}

		if strings.HasPrefix(line, "??") {
			untracked++
			continue
		}

		changed++
	}

	parts := make([]string, 0, 2)
	if changed > 0 {
		parts = append(parts, pluralizeStatusCount(changed, "file changed", "files changed"))
	}
	if untracked > 0 {
		parts = append(parts, pluralizeStatusCount(untracked, "untracked file", "untracked files"))
	}

	if len(parts) == 0 {
		return statusSummaryClean, true
	}

	return strings.Join(parts, ", "), false
}

func pluralizeStatusCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func writeStatusJSON(cmd *cobra.Command, items []statusItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeStatusTable(cmd *cobra.Command, items []statusItem) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	for _, item := range items {
		if _, err := fmt.Fprintf(writer, "[%s]\t%s\n", item.Name, item.Summary); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func resolveStatusPath(ref workspace.Ref, env workspace.EnvVars) (string, error) {
	if strings.TrimSpace(ref.Path) == "" {
		return "", errors.New("ref path cannot be empty")
	}

	resolvedPath, err := workspace.ResolvePath(ref.Path, env)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}
