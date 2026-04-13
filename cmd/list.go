package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jwolf/wsx/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	listStatusOK     = "ok"
	listStatusBroken = "broken"
)

type listItem struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	ResolvedPath string `json:"resolved_path"`
	LinkType     string `json:"link_type"`
	Status       string `json:"status"`
}

func newListCommand() *cobra.Command {
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List linked repositories in the current workspace",
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

			items := make([]listItem, 0, len(loaded.Config.Refs))
			for _, ref := range loaded.Config.Refs {
				items = append(items, buildListItem(loaded.Root, ref, env))
			}

			if jsonOutput {
				return writeListJSON(cmd, items)
			}
			return writeListTable(cmd, items)
		},
	}

	command.Flags().BoolVar(&jsonOutput, "json", false, "Output linked repositories as JSON")
	return command
}

func buildListItem(root string, ref workspace.Ref, env workspace.EnvVars) listItem {
	item := listItem{
		Name:   ref.Name,
		Path:   ref.Path,
		Status: listStatusOK,
	}

	resolvedPath, err := workspace.ResolvePath(ref.Path, env)
	if err == nil {
		item.ResolvedPath = resolvedPath
		if info, statErr := os.Stat(resolvedPath); statErr != nil || !info.IsDir() {
			item.Status = listStatusBroken
		}
	} else {
		item.Status = listStatusBroken
	}

	linkType, err := validateWorkspaceLinkTarget(root, ref.Name, item.ResolvedPath)
	if err == nil {
		item.LinkType = linkType
	} else {
		item.Status = listStatusBroken
	}

	return item
}

func writeListJSON(cmd *cobra.Command, items []listItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, writeErr := cmd.OutOrStdout().Write(data)
	return writeErr
}

func writeListTable(cmd *cobra.Command, items []listItem) error {
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, "NAME\tPATH\tTYPE\tSTATUS"); err != nil {
		return err
	}

	for _, item := range items {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", item.Name, item.Path, item.LinkType, item.Status); err != nil {
			return err
		}
	}

	return writer.Flush()
}
