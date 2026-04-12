package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type RepoDetection struct {
	Language   string   `json:"language"`
	Framework  string   `json:"framework,omitempty"`
	Indicators []string `json:"indicators,omitempty"`
}

func DetectRepo(repoRoot string) (RepoDetection, error) {
	absoluteRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return RepoDetection{}, err
	}

	switch {
	case fileExists(filepath.Join(absoluteRoot, "go.mod")):
		return RepoDetection{
			Language:   "Go",
			Indicators: []string{"go.mod"},
		}, nil
	case fileExists(filepath.Join(absoluteRoot, "Cargo.toml")):
		return RepoDetection{
			Language:   "Rust",
			Indicators: []string{"Cargo.toml"},
		}, nil
	}

	if packageJSONPath := filepath.Join(absoluteRoot, "package.json"); fileExists(packageJSONPath) {
		return detectNodeRepo(packageJSONPath)
	}

	for _, candidate := range []string{"pyproject.toml", "requirements.txt"} {
		path := filepath.Join(absoluteRoot, candidate)
		if !fileExists(path) {
			continue
		}

		detection, err := detectPythonRepo(path, candidate)
		if err != nil {
			return RepoDetection{}, err
		}
		return detection, nil
	}

	return RepoDetection{Language: "Unknown"}, nil
}

func detectNodeRepo(path string) (RepoDetection, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return RepoDetection{}, err
	}

	type packageJSON struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	var pkg packageJSON
	if err := json.Unmarshal(content, &pkg); err != nil {
		return RepoDetection{}, err
	}

	dependencies := make(map[string]struct{}, len(pkg.Dependencies)+len(pkg.DevDependencies))
	for name := range pkg.Dependencies {
		dependencies[strings.ToLower(name)] = struct{}{}
	}
	for name := range pkg.DevDependencies {
		dependencies[strings.ToLower(name)] = struct{}{}
	}

	return RepoDetection{
		Language:   "Node.js",
		Framework:  detectNodeFramework(dependencies),
		Indicators: []string{"package.json"},
	}, nil
}

func detectPythonRepo(path, indicator string) (RepoDetection, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return RepoDetection{}, err
	}

	lower := strings.ToLower(string(content))
	framework := ""
	for _, candidate := range []struct {
		name    string
		markers []string
	}{
		{name: "FastAPI", markers: []string{"fastapi"}},
		{name: "Django", markers: []string{"django"}},
		{name: "Flask", markers: []string{"flask"}},
	} {
		if containsAny(lower, candidate.markers...) {
			framework = candidate.name
			break
		}
	}

	return RepoDetection{
		Language:   "Python",
		Framework:  framework,
		Indicators: []string{indicator},
	}, nil
}

func detectNodeFramework(dependencies map[string]struct{}) string {
	for _, candidate := range []struct {
		name    string
		markers []string
	}{
		{name: "Next.js", markers: []string{"next"}},
		{name: "Nuxt", markers: []string{"nuxt"}},
		{name: "SvelteKit", markers: []string{"@sveltejs/kit"}},
		{name: "NestJS", markers: []string{"@nestjs/core"}},
		{name: "Express", markers: []string{"express"}},
		{name: "React", markers: []string{"react"}},
		{name: "Vue", markers: []string{"vue"}},
	} {
		if hasAnyKey(dependencies, candidate.markers...) {
			return candidate.name
		}
	}

	return ""
}

func hasAnyKey(values map[string]struct{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := values[strings.ToLower(key)]; ok {
			return true
		}
	}

	return false
}

func containsAny(content string, values ...string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.Contains(content, strings.ToLower(value))
	})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
