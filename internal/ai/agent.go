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

type ImportedInstruction struct {
	Path    string
	Content string
}

type WorkspaceInstructionRepo struct {
	Name        string
	Root        string
	Detection   RepoDetection
	Imported    []ImportedInstruction
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

		imported, err := findImportedInstructions(absoluteRoot)
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
			Imported:    imported,
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
	builder.WriteString("- `.wsx.json` stores portable placeholder paths and must not be rewritten to machine-specific absolute paths.\n")
	builder.WriteString("- `.wsx.env` is local-only machine state and should not be committed from the workspace.\n")
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

	builder.WriteString("## Repo-Specific Imported Instructions\n\n")
	if !hasImportedInstructions(instructions.Repos) {
		builder.WriteString("No repo-specific `CLAUDE.md` or `AGENTS.md` files were found in linked repositories.\n\n")
	} else {
		for _, repo := range instructions.Repos {
			builder.WriteString("### Repo: `")
			builder.WriteString(repo.Name)
			builder.WriteString("`\n\n")
			builder.WriteString("This section applies when working in linked repo `")
			builder.WriteString(repo.Name)
			builder.WriteString("`.\n\n")

			if len(repo.Imported) == 0 {
				builder.WriteString("No repo-specific instruction files were found for this repo.\n\n")
				continue
			}

			for _, imported := range repo.Imported {
				builder.WriteString("#### Source: `")
				builder.WriteString(imported.Path)
				builder.WriteString("`\n\n")
				builder.WriteString(imported.Content)
				if !strings.HasSuffix(imported.Content, "\n") {
					builder.WriteString("\n")
				}
				builder.WriteString("\n")
			}
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

func hasImportedInstructions(repos []WorkspaceInstructionRepo) bool {
	for _, repo := range repos {
		if len(repo.Imported) > 0 {
			return true
		}
	}

	return false
}

func findImportedInstructions(repoRoot string) ([]ImportedInstruction, error) {
	paths := make([]string, 0, 4)

	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}

		if isImportedInstructionPath(relativePath) {
			paths = append(paths, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)

	imported := make([]ImportedInstruction, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		relativePath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return nil, err
		}

		imported = append(imported, ImportedInstruction{
			Path:    filepath.ToSlash(relativePath),
			Content: string(content),
		})
	}

	return imported, nil
}

func isImportedInstructionPath(relativePath string) bool {
	normalized := filepath.ToSlash(filepath.Clean(relativePath))
	base := pathBase(normalized)
	if base == "CLAUDE.md" || base == "AGENTS.md" {
		return true
	}

	return false
}

func pathBase(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func WriteWorkspaceInstructionFiles(root string, content string) error {
	files := []string{
		WorkspaceClaudeFilePath,
		WorkspaceAgentsFilePath,
	}

	for _, relativePath := range files {
		targetPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if _, err := os.Stat(targetPath); err == nil {
			return fmt.Errorf("%s already exists", relativePath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", relativePath, err)
		}
	}

	for _, relativePath := range files {
		targetPath := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", relativePath, err)
		}
	}

	return nil
}
