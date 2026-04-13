package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jwolf/wsx/internal/ai"
	"github.com/jwolf/wsx/internal/workspace"
	"github.com/spf13/cobra"
)

func newDumpCommand() *cobra.Command {
	var include string
	var exclude string
	var pathFilter string
	var repoName string
	var allFiles bool
	var noIgnore bool
	var format string
	var maxTokens int
	var dryRun bool

	command := &cobra.Command{
		Use:   "dump",
		Short: "Dump selected file contents across linked repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxTokens < 0 {
				return fmt.Errorf("max-tokens cannot be negative")
			}

			options := ai.DumpOptions{
				IncludeGlobs:   parseCommaPatterns(include),
				ExcludeGlobs:   parseCommaPatterns(exclude),
				PathPrefix:     pathFilter,
				RepoName:       strings.TrimSpace(repoName),
				AllFiles:       allFiles,
				IncludeIgnored: noIgnore,
				MaxTokens:      maxTokens,
				DryRun:         dryRun,
			}

			if err := ai.ValidateDumpScope(options); err != nil {
				return err
			}

			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "markdown"
			}
			if format != "markdown" && format != "json" {
				return fmt.Errorf("unsupported dump format: %s", format)
			}

			loaded, err := workspace.LoadConfig("")
			if err != nil {
				return err
			}

			env, err := loadWorkspaceEnv(loaded.Root)
			if err != nil {
				return err
			}

			repos := make([]ai.DumpRepo, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				resolvedPath, resolveErr := resolveStatusPath(ref, env)
				if resolveErr != nil {
					return resolveErr
				}

				repos = append(repos, ai.DumpRepo{
					Name: ref.Name,
					Root: resolvedPath,
				})
			}

			result, err := ai.DumpWorkspace(loaded.Config.Name, repos, options)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return writeDumpJSON(cmd, result.Files)
			default:
				_, err = cmd.OutOrStdout().Write([]byte(ai.RenderDumpMarkdown(result, dryRun)))
				return err
			}
		},
	}

	command.Flags().StringVar(&include, "include", "", "Comma-separated glob patterns to include")
	command.Flags().StringVar(&exclude, "exclude", "", "Comma-separated glob patterns to exclude")
	command.Flags().StringVar(&pathFilter, "path", "", "Only dump files under this relative path within each repository")
	command.Flags().StringVar(&repoName, "repo", "", "Only dump files from one linked repository")
	command.Flags().BoolVar(&allFiles, "all-files", false, "Dump all files without requiring a narrowing filter")
	command.Flags().BoolVar(&noIgnore, "no-ignore", false, "Ignore .gitignore rules but still skip built-in noise files")
	command.Flags().StringVar(&format, "format", "markdown", "Output format: markdown or json")
	command.Flags().IntVar(&maxTokens, "max-tokens", 0, "Truncate output when estimated token count exceeds this limit")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "List matched files without printing file contents")
	return command
}

func writeDumpJSON(cmd *cobra.Command, files []ai.DumpFile) error {
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}
