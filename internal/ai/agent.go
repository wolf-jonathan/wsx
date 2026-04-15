package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	WorkspaceClaudeFilePath = "CLAUDE.md"
	WorkspaceAgentsFilePath = "AGENTS.md"
)

type InstructionRepo struct {
	Name string
	Root string
}

type InstructionReference struct {
	Path string
}

type WorkspaceInstructionRepo struct {
	Name        string
	Root        string
	Detection   RepoDetection
	References  []InstructionReference
	DisplayRoot string
}

type WorkspaceInstructions struct {
	WorkspaceName string
	Purpose       string
	Tree          string
	Repos         []WorkspaceInstructionRepo
}

func GenerateWorkspaceInstructions(workspaceName, purpose string, repos []InstructionRepo) (WorkspaceInstructions, error) {
	treeRepos := make([]TreeRepo, 0, len(repos))
	instructionRepos := make([]WorkspaceInstructionRepo, 0, len(repos))

	for _, repo := range repos {
		absoluteRoot, err := filepath.Abs(repo.Root)
		if err != nil {
			return WorkspaceInstructions{}, err
		}

		detection, err := DetectRepo(absoluteRoot)
		if err != nil {
			return WorkspaceInstructions{}, err
		}

		references, err := findInstructionReferences(absoluteRoot)
		if err != nil {
			return WorkspaceInstructions{}, err
		}

		treeRepos = append(treeRepos, TreeRepo{
			Name: repo.Name,
			Root: absoluteRoot,
		})
		instructionRepos = append(instructionRepos, WorkspaceInstructionRepo{
			Name:        repo.Name,
			Root:        absoluteRoot,
			DisplayRoot: filepath.ToSlash(absoluteRoot),
			Detection:   detection,
			References:  references,
		})
	}

	tree, err := RenderWorkspaceTree(workspaceName, treeRepos, TreeOptions{MaxDepth: 2})
	if err != nil {
		return WorkspaceInstructions{}, err
	}

	return WorkspaceInstructions{
		WorkspaceName: workspaceName,
		Purpose:       strings.TrimSpace(purpose),
		Tree:          tree,
		Repos:         instructionRepos,
	}, nil
}

func BuildWorkspaceInstructionContent(workspaceName, purpose string, repos []InstructionRepo) (string, error) {
	instructions, err := GenerateWorkspaceInstructions(workspaceName, purpose, repos)
	if err != nil {
		return "", err
	}

	return RenderWorkspaceInstructions(instructions), nil
}

func RenderWorkspaceInstructions(instructions WorkspaceInstructions) string {
	var builder strings.Builder

	builder.WriteString("# Workspace Instructions\n\n")
	builder.WriteString("This workspace is named `")
	builder.WriteString(instructions.WorkspaceName)
	builder.WriteString("`.\n\n")

	if instructions.Purpose != "" {
		builder.WriteString("Purpose: ")
		builder.WriteString(instructions.Purpose)
		builder.WriteString("\n\n")
	}

	builder.WriteString("The workspace root contains linked repositories managed by `wsx`. Treat those directories as links to real repos elsewhere on disk, not copied sources.\n\n")

	builder.WriteString("## Workspace Rules\n\n")
	builder.WriteString("- `.wsx.json` stores absolute local paths for linked repositories and should stay gitignored in personal workspaces.\n")
	builder.WriteString("- Linked repo directories in the workspace root may be symlinks or junctions depending on platform and permissions.\n")
	builder.WriteString("- When a repo-specific section below applies to the repo you are editing, follow that section in addition to these workspace-wide rules.\n\n")

	builder.WriteString("## Linked Repositories\n\n")
	for _, repo := range instructions.Repos {
		builder.WriteString("- `")
		builder.WriteString(repo.Name)
		builder.WriteString("` (")
		builder.WriteString(repo.DisplayRoot)
		builder.WriteString(") - ")
		builder.WriteString(formatRepoLabel(repo.Detection))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	builder.WriteString("## Workspace Tree\n\n```text\n")
	builder.WriteString(instructions.Tree)
	builder.WriteString("```\n\n")

	builder.WriteString("## Repo Instruction References\n\n")
	if !hasInstructionReferences(instructions.Repos) {
		builder.WriteString("No repo-specific instruction files were found in linked repositories.\n\n")
	} else {
		for _, repo := range instructions.Repos {
			builder.WriteString("### Repo: `")
			builder.WriteString(repo.Name)
			builder.WriteString("`\n\n")
			builder.WriteString("This section lists instruction file references only. Contents are not duplicated here.\n\n")

			if len(repo.References) == 0 {
				builder.WriteString("No repo-specific instruction files were found for this repo.\n\n")
				continue
			}

			for _, reference := range repo.References {
				builder.WriteString("- `")
				builder.WriteString(reference.Path)
				builder.WriteString("`\n")
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("## Custom Notes\n\n")
	builder.WriteString("Add workspace-specific notes here.\n")

	return builder.String()
}

func formatRepoLabel(detection RepoDetection) string {
	label := detection.Language
	if strings.TrimSpace(label) == "" {
		label = "Unknown"
	}
	if strings.TrimSpace(detection.Framework) != "" {
		label += " / " + detection.Framework
	}
	return label
}

func hasInstructionReferences(repos []WorkspaceInstructionRepo) bool {
	for _, repo := range repos {
		if len(repo.References) > 0 {
			return true
		}
	}

	return false
}

func findInstructionReferences(repoRoot string) ([]InstructionReference, error) {
	references := make([]InstructionReference, 0, 3)

	if err := collectInstructionReferences(repoRoot, repoRoot, &references); err != nil {
		return nil, err
	}

	sort.Slice(references, func(i, j int) bool {
		return references[i].Path < references[j].Path
	})

	return references, nil
}

func collectInstructionReferences(repoRoot, currentDir string, references *[]InstructionReference) error {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		path := filepath.Join(currentDir, entry.Name())
		if entry.IsDir() {
			if entry.Name() == ".git" {
				continue
			}
			if err := collectInstructionReferences(repoRoot, path, references); err != nil {
				return err
			}
			continue
		}

		relativePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if !isWorkspaceInstructionReferencePath(relativePath) {
			continue
		}

		*references = append(*references, InstructionReference{Path: relativePath})
	}

	return nil
}

func isWorkspaceInstructionReferencePath(relativePath string) bool {
	switch {
	case relativePath == ".github/copilot-instructions.md":
		return true
	case filepath.Base(relativePath) == WorkspaceAgentsFilePath:
		return true
	case filepath.Base(relativePath) == WorkspaceClaudeFilePath:
		return true
	default:
		return false
	}
}

func WriteWorkspaceInstructionFiles(root string, content string) ([]string, error) {
	files := []string{
		WorkspaceClaudeFilePath,
		WorkspaceAgentsFilePath,
	}

	overwritten := make([]string, 0, len(files))
	for _, relativePath := range files {
		targetPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if _, err := os.Stat(targetPath); err == nil {
			overwritten = append(overwritten, relativePath)
		} else if !os.IsNotExist(err) {
			return overwritten, fmt.Errorf("stat %s: %w", relativePath, err)
		}
	}

	for _, relativePath := range files {
		targetPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return overwritten, fmt.Errorf("create %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return overwritten, fmt.Errorf("write %s: %w", relativePath, err)
		}
	}

	return overwritten, nil
}
