package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	wsxgit "github.com/wolf-jonathan/workspace-x/internal/git"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
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
	var parallel bool

	command := &cobra.Command{
		Use:   "status",
		Short: "Run git status across linked repositories",
		Args:  cobra.NoArgs,
		Example: `wsx status
wsx status --parallel --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			client := newStatusGitClient()
			items := make([]statusItem, len(loaded.Config.Refs))

			if parallel {
				runStatusesInParallel(loaded.Config.Refs, client, items)
			} else {
				runStatusesSequentially(loaded.Config.Refs, client, items)
			}

			hasIssues := false
			for _, item := range items {
				if item.Error != "" || !item.Clean {
					hasIssues = true
					break
				}
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
	command.Flags().BoolVar(&parallel, "parallel", false, "Run git status in parallel across repositories")
	return command
}

func runStatusesSequentially(refs []workspace.Ref, client gitStatusClient, items []statusItem) {
	for index, ref := range refs {
		items[index] = statusRef(ref, client)
	}
}

func runStatusesInParallel(refs []workspace.Ref, client gitStatusClient, items []statusItem) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(refs))

	for index, ref := range refs {
		go func(index int, ref workspace.Ref) {
			defer waitGroup.Done()
			items[index] = statusRef(ref, client)
		}(index, ref)
	}

	waitGroup.Wait()
}

func statusRef(ref workspace.Ref, client gitStatusClient) statusItem {
	item := statusItem{
		Name:  ref.Name,
		Path:  ref.Path,
		Clean: false,
	}

	resolvedPath, resolveErr := resolveStatusPath(ref)
	if resolveErr != nil {
		item.Error = resolveErr.Error()
		item.Summary = "error: " + resolveErr.Error()
		return item
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
		return item
	}

	item.Summary, item.Clean = summarizeGitStatus(result.Stdout)
	return item
}

func summarizeGitStatus(stdout string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n")
	branchSummary := ""
	changed := 0
	untracked := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "##") {
			branchSummary = strings.TrimSpace(strings.TrimPrefix(line, "##"))
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
		return formatStatusSummary(branchSummary, statusSummaryClean), true
	}

	return formatStatusSummary(branchSummary, strings.Join(parts, ", ")), false
}

func pluralizeStatusCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func formatStatusSummary(branchSummary, summary string) string {
	if branchSummary == "" {
		return summary
	}
	return branchSummary + "; " + summary
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

func resolveStatusPath(ref workspace.Ref) (string, error) {
	resolvedPath, err := workspace.ResolveStoredPath(ref.Path)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}
