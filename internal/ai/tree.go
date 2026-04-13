package ai

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TreeRepo struct {
	Name string
	Root string
}

type TreeOptions struct {
	MaxDepth       int
	IncludeIgnored bool
}

type treeWalker struct {
	options TreeOptions
}

var treeDefaultExcludedDirs = map[string]struct{}{
	"dist":         {},
	"node_modules": {},
	"vendor":       {},
}

func RenderWorkspaceTree(workspaceName string, repos []TreeRepo, options TreeOptions) (string, error) {
	var builder strings.Builder
	builder.WriteString(workspaceName)
	builder.WriteString("/\n")

	walker := treeWalker{options: options}
	for index, repo := range repos {
		branch := "├── "
		childPrefix := "│   "
		if index == len(repos)-1 {
			branch = "└── "
			childPrefix = "    "
		}

		if err := walker.renderRepo(&builder, repo, branch, childPrefix); err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

func (w treeWalker) renderRepo(builder *strings.Builder, repo TreeRepo, branch, childPrefix string) error {
	root, err := filepath.Abs(repo.Root)
	if err != nil {
		return err
	}

	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{Op: "tree", Path: root, Err: os.ErrInvalid}
	}

	builder.WriteString(branch)
	builder.WriteString(repo.Name)
	builder.WriteString("/\n")

	var matcher *IgnoreMatcher
	if !w.options.IncludeIgnored {
		matcher, err = LoadIgnoreMatcher(root)
		if err != nil {
			return err
		}
	}

	return w.renderDir(builder, root, root, childPrefix, 0, matcher)
}

func (w treeWalker) renderDir(builder *strings.Builder, repoRoot, currentDir, prefix string, depth int, matcher *IgnoreMatcher) error {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return err
	}

	filtered := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(currentDir, entry.Name())
		if w.shouldSkip(repoRoot, path, entry, matcher) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		leftDir := filtered[i].IsDir()
		rightDir := filtered[j].IsDir()
		if leftDir != rightDir {
			return leftDir
		}
		return strings.ToLower(filtered[i].Name()) < strings.ToLower(filtered[j].Name())
	})

	if w.options.MaxDepth > 0 && depth >= w.options.MaxDepth {
		if len(filtered) > 0 {
			builder.WriteString(prefix)
			builder.WriteString("└── ...\n")
		}
		return nil
	}

	for index, entry := range filtered {
		path := filepath.Join(currentDir, entry.Name())
		branch := "├── "
		nextPrefix := prefix + "│   "
		if index == len(filtered)-1 {
			branch = "└── "
			nextPrefix = prefix + "    "
		}

		builder.WriteString(prefix)
		builder.WriteString(branch)
		builder.WriteString(entry.Name())
		if entry.IsDir() {
			builder.WriteString("/")
		}
		builder.WriteString("\n")

		if entry.IsDir() {
			if err := w.renderDir(builder, repoRoot, path, nextPrefix, depth+1, matcher); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w treeWalker) shouldSkip(repoRoot, path string, entry os.DirEntry, matcher *IgnoreMatcher) bool {
	if !w.options.IncludeIgnored {
		if matcher != nil && matcher.MatchesPath(path) {
			return true
		}
		if entry.IsDir() {
			if _, excluded := treeDefaultExcludedDirs[strings.ToLower(entry.Name())]; excluded {
				return true
			}
		}
	}

	relativePath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return false
	}
	relativePath = filepath.ToSlash(relativePath)
	return relativePath == "."
}
