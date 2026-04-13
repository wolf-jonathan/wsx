package ai

import (
	"strings"
)

type PromptRepo struct {
	Name      string
	Root      string
	Detection RepoDetection
}

type WorkspacePrompt struct {
	WorkspaceName string
	Repos         []PromptRepo
	Tree          string
}

func GenerateWorkspacePrompt(workspaceName string, repos []PromptRepo) WorkspacePrompt {
	return WorkspacePrompt{
		WorkspaceName: workspaceName,
		Repos:         repos,
	}
}

func RenderWorkspacePrompt(prompt WorkspacePrompt) string {
	var builder strings.Builder

	builder.WriteString("You are working in a multi-repo workspace called \"")
	builder.WriteString(prompt.WorkspaceName)
	builder.WriteString("\".\n")
	builder.WriteString("It contains the following repositories:\n\n")

	for _, repo := range prompt.Repos {
		builder.WriteString("- ")
		builder.WriteString(repo.Name)
		builder.WriteString(" (")
		builder.WriteString(repo.Root)
		builder.WriteString(") - ")
		builder.WriteString(formatPromptRepoLabel(repo.Detection))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString("These repository directories are linked into the workspace by `wsx`, so treat them as linked repos rather than copied source trees.\n\n")
	builder.WriteString("Directory structure:\n")
	builder.WriteString(prompt.Tree)

	return builder.String()
}

func formatPromptRepoLabel(detection RepoDetection) string {
	label := strings.TrimSpace(detection.Language)
	if label == "" {
		label = "Unknown"
	}

	if framework := strings.TrimSpace(detection.Framework); framework != "" {
		label += " / " + framework
	}

	return label
}
