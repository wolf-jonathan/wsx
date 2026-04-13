package ai

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type GrepRepo struct {
	Name string
	Root string
}

type GrepOptions struct {
	IncludeGlobs []string
	ExcludeGlobs []string
	ContextLines int
}

type GrepMatch struct {
	Repo   string   `json:"repo"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Match  string   `json:"match"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

func GrepWorkspace(repos []GrepRepo, pattern string, options GrepOptions) ([]GrepMatch, error) {
	matches := make([]GrepMatch, 0, 16)

	for _, repo := range repos {
		repoMatches, err := grepRepo(repo, pattern, options)
		if err != nil {
			return nil, err
		}
		matches = append(matches, repoMatches...)
	}

	return matches, nil
}

func grepRepo(repo GrepRepo, pattern string, options GrepOptions) ([]GrepMatch, error) {
	root, err := filepath.Abs(repo.Root)
	if err != nil {
		return nil, err
	}

	matcher, err := LoadIgnoreMatcher(root)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, 32)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		if matcher.MatchesPath(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		if !shouldSearchPath(relativePath, options) {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)

	matches := make([]GrepMatch, 0, 8)
	for _, path := range paths {
		fileMatches, err := grepFile(repo.Name, root, path, pattern, options.ContextLines)
		if err != nil {
			return nil, err
		}
		matches = append(matches, fileMatches...)
	}

	return matches, nil
}

func shouldSearchPath(relativePath string, options GrepOptions) bool {
	relativePath = filepath.ToSlash(relativePath)
	base := filepath.Base(relativePath)

	if len(options.IncludeGlobs) > 0 && !matchesAnyGlob(relativePath, base, options.IncludeGlobs) {
		return false
	}

	if len(options.ExcludeGlobs) > 0 && matchesAnyGlob(relativePath, base, options.ExcludeGlobs) {
		return false
	}

	return true
}

func matchesAnyGlob(relativePath, base string, globs []string) bool {
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

func grepFile(repoName, repoRoot, path, pattern string, contextLines int) ([]GrepMatch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return nil, nil
	}

	relativePath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return nil, err
	}
	relativePath = filepath.ToSlash(relativePath)

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")

	matches := make([]GrepMatch, 0, 4)
	for index, line := range lines {
		if !strings.Contains(line, pattern) {
			continue
		}

		match := GrepMatch{
			Repo:  repoName,
			File:  relativePath,
			Line:  index + 1,
			Match: line,
		}

		if contextLines > 0 {
			start := index - contextLines
			if start < 0 {
				start = 0
			}
			end := index + contextLines + 1
			if end > len(lines) {
				end = len(lines)
			}

			match.Before = append(match.Before, lines[start:index]...)
			match.After = append(match.After, lines[index+1:end]...)
		}

		matches = append(matches, match)
	}

	return matches, nil
}
