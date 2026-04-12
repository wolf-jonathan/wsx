package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	wsxgit "github.com/jwolf/wsx/internal/git"
	"github.com/jwolf/wsx/internal/workspace"
	"github.com/spf13/cobra"
)

type gitFetchClient interface {
	Fetch(path string) (wsxgit.CommandResult, error)
}

var newFetchGitClient = func() gitFetchClient {
	return wsxgit.NewClient(nil)
}

type fetchItem struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	ResolvedPath string `json:"resolved_path"`
	Summary      string `json:"summary"`
	Error        string `json:"error,omitempty"`
	ExitCode     int    `json:"exit_code"`
}

type fetchCommandError struct {
	message string
}

func (e *fetchCommandError) Error() string {
	return e.message
}

func newFetchCommand() *cobra.Command {
	var jsonOutput bool
	var parallel bool

	command := &cobra.Command{
		Use:   "fetch",
		Short: "Run git fetch across linked repositories",
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

			client := newFetchGitClient()
			items := make([]fetchItem, len(loaded.Config.Refs))

			if parallel {
				runFetchesInParallel(loaded.Config.Refs, env, client, items)
			} else {
				runFetchesSequentially(loaded.Config.Refs, env, client, items)
			}

			hasFailures := false
			for _, item := range items {
				if item.Error != "" {
					hasFailures = true
					break
				}
			}

			if jsonOutput {
				if err := writeFetchJSON(cmd, items); err != nil {
					return err
				}
			} else {
				if err := writeFetchTable(cmd, items); err != nil {
					return err
				}
			}

			if hasFailures {
				return &fetchCommandError{message: "one or more repositories could not be fetched"}
			}

			return nil
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output fetch results as JSON")
	command.Flags().BoolVar(&parallel, "parallel", false, "Fetch repositories in parallel")
	return command
}

func runFetchesSequentially(refs []workspace.Ref, env workspace.EnvVars, client gitFetchClient, items []fetchItem) {
	for index, ref := range refs {
		items[index] = fetchRef(ref, env, client)
	}
}

func runFetchesInParallel(refs []workspace.Ref, env workspace.EnvVars, client gitFetchClient, items []fetchItem) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(refs))

	for index, ref := range refs {
		go func(index int, ref workspace.Ref) {
			defer waitGroup.Done()
			items[index] = fetchRef(ref, env, client)
		}(index, ref)
	}

	waitGroup.Wait()
}

func fetchRef(ref workspace.Ref, env workspace.EnvVars, client gitFetchClient) fetchItem {
	item := fetchItem{
		Name: ref.Name,
		Path: ref.Path,
	}

	resolvedPath, resolveErr := resolveFetchPath(ref, env)
	if resolveErr != nil {
		item.Error = resolveErr.Error()
		item.Summary = "error: " + resolveErr.Error()
		return item
	}

	item.ResolvedPath = resolvedPath

	result, fetchErr := client.Fetch(resolvedPath)
	item.ExitCode = result.ExitCode
	if fetchErr != nil {
		item.Error = fetchErr.Error()
		item.Summary = "error: " + summarizeFetchOutput(result.Stdout, result.Stderr, fetchErr)
		return item
	}

	item.Summary = summarizeFetchOutput(result.Stdout, result.Stderr, nil)
	return item
}

func summarizeFetchOutput(stdout, stderr string, fallback error) string {
	lines := make([]string, 0, 4)
	for _, stream := range []string{stdout, stderr} {
		for _, line := range strings.Split(strings.ReplaceAll(stream, "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lines = append(lines, line)
		}
	}

	if len(lines) > 0 {
		return strings.Join(lines, " | ")
	}

	if fallback != nil {
		return fallback.Error()
	}

	return "fetched"
}

func writeFetchJSON(cmd *cobra.Command, items []fetchItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeFetchTable(cmd *cobra.Command, items []fetchItem) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	for _, item := range items {
		if _, err := fmt.Fprintf(writer, "[%s]\t%s\n", item.Name, item.Summary); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func resolveFetchPath(ref workspace.Ref, env workspace.EnvVars) (string, error) {
	if strings.TrimSpace(ref.Path) == "" {
		return "", errors.New("ref path cannot be empty")
	}

	resolvedPath, err := workspace.ResolvePath(ref.Path, env)
	if err != nil {
		return "", err
	}

	return filepath.Clean(resolvedPath), nil
}
