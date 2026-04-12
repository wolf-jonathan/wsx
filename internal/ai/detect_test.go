package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRepoDetectsGoModule(t *testing.T) {
	repoRoot := t.TempDir()
	writeDetectFile(t, filepath.Join(repoRoot, "go.mod"), "module example.com/service\n\ngo 1.22.0\n")

	result, err := DetectRepo(repoRoot)
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if result.Language != "Go" {
		t.Fatalf("result.Language = %q, want Go", result.Language)
	}
	if result.Framework != "" {
		t.Fatalf("result.Framework = %q, want empty", result.Framework)
	}
	if len(result.Indicators) != 1 || result.Indicators[0] != "go.mod" {
		t.Fatalf("result.Indicators = %#v, want [go.mod]", result.Indicators)
	}
}

func TestDetectRepoDetectsNextJSFromPackageJSON(t *testing.T) {
	repoRoot := t.TempDir()
	writeDetectFile(t, filepath.Join(repoRoot, "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "next": "14.2.0",
    "react": "18.3.1"
  }
}
`)

	result, err := DetectRepo(repoRoot)
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if result.Language != "Node.js" {
		t.Fatalf("result.Language = %q, want Node.js", result.Language)
	}
	if result.Framework != "Next.js" {
		t.Fatalf("result.Framework = %q, want Next.js", result.Framework)
	}
	if len(result.Indicators) == 0 || result.Indicators[0] != "package.json" {
		t.Fatalf("result.Indicators = %#v, want package.json evidence", result.Indicators)
	}
}

func TestDetectRepoDetectsPythonFrameworkFromPyproject(t *testing.T) {
	repoRoot := t.TempDir()
	writeDetectFile(t, filepath.Join(repoRoot, "pyproject.toml"), `[project]
name = "api"
dependencies = ["fastapi>=0.111.0", "uvicorn>=0.30.0"]
`)

	result, err := DetectRepo(repoRoot)
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if result.Language != "Python" {
		t.Fatalf("result.Language = %q, want Python", result.Language)
	}
	if result.Framework != "FastAPI" {
		t.Fatalf("result.Framework = %q, want FastAPI", result.Framework)
	}
}

func TestDetectRepoDetectsRustFromCargoToml(t *testing.T) {
	repoRoot := t.TempDir()
	writeDetectFile(t, filepath.Join(repoRoot, "Cargo.toml"), `[package]
name = "workspace-tool"
version = "0.1.0"
edition = "2021"
`)

	result, err := DetectRepo(repoRoot)
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if result.Language != "Rust" {
		t.Fatalf("result.Language = %q, want Rust", result.Language)
	}
	if result.Framework != "" {
		t.Fatalf("result.Framework = %q, want empty", result.Framework)
	}
}

func TestDetectRepoFallsBackToUnknownWhenNoMarkersExist(t *testing.T) {
	repoRoot := t.TempDir()

	result, err := DetectRepo(repoRoot)
	if err != nil {
		t.Fatalf("DetectRepo() error = %v", err)
	}

	if result.Language != "Unknown" {
		t.Fatalf("result.Language = %q, want Unknown", result.Language)
	}
	if result.Framework != "" {
		t.Fatalf("result.Framework = %q, want empty", result.Framework)
	}
	if len(result.Indicators) != 0 {
		t.Fatalf("result.Indicators = %#v, want empty", result.Indicators)
	}
}

func writeDetectFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
