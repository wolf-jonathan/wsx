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

type commandRunner interface {
	Run(dir, name string, args ...string) (wsxgit.CommandResult, error)
}

var newExecRunner = func() commandRunner {
	return wsxgit.ExecRunner{}
}

type execItem struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	ResolvedPath string   `json:"resolved_path"`
	Command      []string `json:"command"`
	Stdout       string   `json:"stdout"`
	Stderr       string   `json:"stderr"`
	Error        string   `json:"error,omitempty"`
	ExitCode     int      `json:"exit_code"`
}

type execCommandError struct {
	message string
}

func (e *execCommandError) Error() string {
	return e.message
}

func newExecCommand() *cobra.Command {
	var jsonOutput bool
	var parallel bool

	command := &cobra.Command{
		Use:   "exec -- <cmd>",
		Short: "Run a command across linked repositories",
		Args:  cobra.MinimumNArgs(1),
		Example: `wsx exec -- git status
wsx exec --parallel -- npm test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			runner := newExecRunner()
			items := make([]execItem, len(loaded.Config.Refs))

			if parallel {
				runExecsInParallel(loaded.Config.Refs, runner, args, items)
			} else {
				runExecsSequentially(loaded.Config.Refs, runner, args, items)
			}

			hasFailures := false
			for _, item := range items {
				if execItemFailed(item) {
					hasFailures = true
					break
				}
			}

			if jsonOutput {
				if err := writeExecJSON(cmd, items); err != nil {
					return err
				}
			} else {
				if err := writeExecText(cmd, items); err != nil {
					return err
				}
			}

			if hasFailures {
				return &execCommandError{message: "one or more repositories could not execute the command"}
			}

			return nil
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output exec results as JSON")
	command.Flags().BoolVar(&parallel, "parallel", false, "Run the command in parallel across repositories")
	return command
}

func runExecsSequentially(refs []workspace.Ref, runner commandRunner, commandArgs []string, items []execItem) {
	for index, ref := range refs {
		items[index] = execRef(ref, runner, commandArgs)
	}
}

func runExecsInParallel(refs []workspace.Ref, runner commandRunner, commandArgs []string, items []execItem) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(refs))

	for index, ref := range refs {
		go func(index int, ref workspace.Ref) {
			defer waitGroup.Done()
			items[index] = execRef(ref, runner, commandArgs)
		}(index, ref)
	}

	waitGroup.Wait()
}

func execRef(ref workspace.Ref, runner commandRunner, commandArgs []string) execItem {
	item := execItem{
		Name:    ref.Name,
		Path:    ref.Path,
		Command: append([]string(nil), commandArgs...),
	}

	resolvedPath, resolveErr := resolveExecPath(ref)
	if resolveErr != nil {
		item.Error = resolveErr.Error()
		item.Stderr = resolveErr.Error()
		return item
	}

	item.ResolvedPath = resolvedPath

	result, runErr := runner.Run(resolvedPath, commandArgs[0], commandArgs[1:]...)
	item.Stdout = result.Stdout
	item.Stderr = result.Stderr
	item.ExitCode = result.ExitCode
	if runErr != nil {
		item.Error = runErr.Error()
		return item
	}

	return item
}

func writeExecJSON(cmd *cobra.Command, items []execItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeExecText(cmd *cobra.Command, items []execItem) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	for _, item := range items {
		if _, err := fmt.Fprintf(writer, "[%s]\n", item.Name); err != nil {
			return err
		}

		output := combinedExecOutput(item)
		for _, line := range strings.Split(output, "\n") {
			if _, err := fmt.Fprintf(writer, "  %s\n", line); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func combinedExecOutput(item execItem) string {
	lines := make([]string, 0, 8)
	for _, stream := range []string{item.Stdout, item.Stderr} {
		for _, line := range strings.Split(strings.ReplaceAll(stream, "\r\n", "\n"), "\n") {
			line = strings.TrimRight(line, " \t")
			if line == "" {
				continue
			}
			lines = append(lines, line)
		}
	}

	if item.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("command failed with exit code %d", item.ExitCode))
	}

	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	if item.Error != "" {
		return item.Error
	}

	return "(no output)"
}

func execItemFailed(item execItem) bool {
	return item.Error != "" || item.ExitCode != 0
}

func resolveExecPath(ref workspace.Ref) (string, error) {
	resolvedPath, err := workspace.ResolveStoredPath(ref.Path)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}
