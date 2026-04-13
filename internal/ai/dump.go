package ai

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

type DumpRepo struct {
	Name string
	Root string
}

type DumpOptions struct {
	IncludeGlobs   []string
	ExcludeGlobs   []string
	PathPrefix     string
	RepoName       string
	AllFiles       bool
	IncludeIgnored bool
	MaxTokens      int
	DryRun         bool
}

type DumpFile struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Content string `json:"content,omitempty"`
}

type DumpResult struct {
	Workspace       string
	Files           []DumpFile
	EstimatedTokens int
	Truncated       bool
}

var dumpBuiltInMatcher = ignore.CompileIgnoreLines(builtInIgnoreLines...)

func DumpWorkspace(workspaceName string, repos []DumpRepo, options DumpOptions) (DumpResult, error) {
	result := DumpResult{
		Workspace: workspaceName,
		Files:     make([]DumpFile, 0, 16),
	}

	for _, repo := range repos {
		if options.RepoName != "" && repo.Name != options.RepoName {
			continue
		}

		files, estimatedTokens, truncated, err := dumpRepo(repo, options, result.EstimatedTokens)
		if err != nil {
			return DumpResult{}, err
		}

		result.Files = append(result.Files, files...)
		result.EstimatedTokens = estimatedTokens
		if truncated {
			result.Truncated = true
			break
		}
	}

	return result, nil
}

func RenderDumpMarkdown(result DumpResult, dryRun bool) string {
	var builder strings.Builder
	builder.WriteString("# Workspace: ")
	builder.WriteString(result.Workspace)
	builder.WriteString("\n")

	if result.Truncated {
		builder.WriteString("\nWarning: output truncated because estimated token count exceeded the configured limit.\n")
	}

	for _, file := range result.Files {
		builder.WriteString("\n## ")
		builder.WriteString(file.Repo)
		builder.WriteString("/")
		builder.WriteString(file.File)
		builder.WriteString("\n")

		if dryRun {
			continue
		}

		builder.WriteString("```")
		builder.WriteString(detectFenceLanguage(file.File))
		builder.WriteString("\n")
		builder.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("```\n")
	}

	return builder.String()
}

func dumpRepo(repo DumpRepo, options DumpOptions, startingTokens int) ([]DumpFile, int, bool, error) {
	root, err := filepath.Abs(repo.Root)
	if err != nil {
		return nil, startingTokens, false, err
	}

	var matcher *IgnoreMatcher
	if !options.IncludeIgnored {
		matcher, err = LoadIgnoreMatcher(root)
		if err != nil {
			return nil, startingTokens, false, err
		}
	}

	paths := make([]string, 0, 32)
	err = filepath.WalkDir(root, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == root {
			return nil
		}

		relativePath, relErr := filepath.Rel(root, currentPath)
		if relErr != nil {
			return relErr
		}
		relativePath = filepath.ToSlash(relativePath)

		if shouldSkipDumpPath(currentPath, relativePath, entry, matcher, options.IncludeIgnored) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			return nil
		}
		if !shouldDumpPath(relativePath, options) {
			return nil
		}

		paths = append(paths, currentPath)
		return nil
	})
	if err != nil {
		return nil, startingTokens, false, err
	}

	sort.Strings(paths)

	files := make([]DumpFile, 0, len(paths))
	estimatedTokens := startingTokens
	for _, currentPath := range paths {
		relativePath, relErr := filepath.Rel(root, currentPath)
		if relErr != nil {
			return nil, estimatedTokens, false, relErr
		}
		relativePath = filepath.ToSlash(relativePath)

		file := DumpFile{
			Repo: repo.Name,
			File: relativePath,
		}

		if !options.DryRun {
			data, readErr := os.ReadFile(currentPath)
			if readErr != nil {
				return nil, estimatedTokens, false, readErr
			}
			if bytes.IndexByte(data, 0) >= 0 {
				continue
			}

			file.Content = string(data)
		}

		nextTokens := estimatedTokens + estimateDumpTokens(file.File) + estimateDumpTokens(file.Content)
		if options.MaxTokens > 0 && nextTokens > options.MaxTokens {
			if len(files) == 0 {
				files = append(files, file)
				return files, nextTokens, true, nil
			}
			return files, estimatedTokens, true, nil
		}

		files = append(files, file)
		estimatedTokens = nextTokens
	}

	return files, estimatedTokens, false, nil
}

func shouldSkipDumpPath(fullPath, relativePath string, entry os.DirEntry, matcher *IgnoreMatcher, includeIgnored bool) bool {
	if dumpBuiltInMatcher.MatchesPath(relativePath) {
		return true
	}

	if includeIgnored {
		return false
	}

	if matcher == nil {
		return false
	}

	return matcher.MatchesPath(fullPath)
}

func shouldDumpPath(relativePath string, options DumpOptions) bool {
	relativePath = filepath.ToSlash(relativePath)
	base := filepath.Base(relativePath)

	if normalizedPrefix := normalizeDumpPathPrefix(options.PathPrefix); normalizedPrefix != "" {
		if relativePath != normalizedPrefix && !strings.HasPrefix(relativePath, normalizedPrefix+"/") {
			return false
		}
	}

	if !options.AllFiles && len(options.IncludeGlobs) > 0 && !matchesDumpGlob(relativePath, base, options.IncludeGlobs) {
		return false
	}

	if len(options.ExcludeGlobs) > 0 && matchesDumpGlob(relativePath, base, options.ExcludeGlobs) {
		return false
	}

	return true
}

func matchesDumpGlob(relativePath, base string, globs []string) bool {
	for _, glob := range globs {
		if glob == "" {
			continue
		}

		if strings.Contains(glob, "/") {
			if ok, _ := path.Match(glob, relativePath); ok {
				return true
			}
			continue
		}

		if ok, _ := path.Match(glob, base); ok {
			return true
		}
		if ok, _ := path.Match(glob, relativePath); ok {
			return true
		}
	}

	return false
}

func normalizeDumpPathPrefix(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.Trim(value, "/")
	return value
}

func detectFenceLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "ts"
	case ".tsx":
		return "tsx"
	case ".js":
		return "js"
	case ".jsx":
		return "jsx"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "md"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".sh":
		return "sh"
	default:
		return ""
	}
}

func estimateDumpTokens(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	return (len([]rune(trimmed)) + 3) / 4
}

func ValidateDumpScope(options DumpOptions) error {
	if options.AllFiles {
		return nil
	}
	if len(options.IncludeGlobs) > 0 {
		return nil
	}
	if normalizeDumpPathPrefix(options.PathPrefix) != "" {
		return nil
	}
	if strings.TrimSpace(options.RepoName) != "" {
		return nil
	}

	return fmt.Errorf("wsx dump requires a filter to avoid overwhelming AI context.\nUse --include, --path, or --repo to narrow the output.\nRun with --all-files to override (not recommended for large repos).\n\nTip: try 'wsx tree' first to see what's in your workspace.")
}
