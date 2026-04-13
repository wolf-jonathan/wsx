package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGrepWorkspaceRespectsIgnoresAndGlobFilters(t *testing.T) {
	repoRoot := t.TempDir()
	writeGrepFile(t, filepath.Join(repoRoot, ".gitignore"), "ignored.txt\n")
	writeGrepFile(t, filepath.Join(repoRoot, "src", "auth.go"), "func handleAuth() {}\n")
	writeGrepFile(t, filepath.Join(repoRoot, "src", "notes.txt"), "handleAuth is documented here\n")
	writeGrepFile(t, filepath.Join(repoRoot, "ignored.txt"), "handleAuth should stay hidden\n")

	matches, err := GrepWorkspace([]GrepRepo{
		{Name: "auth-service", Root: repoRoot},
	}, "handleAuth", GrepOptions{
		IncludeGlobs: []string{"*.go", "*.txt"},
		ExcludeGlobs: []string{"notes.txt"},
	})
	if err != nil {
		t.Fatalf("GrepWorkspace() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}

	match := matches[0]
	if match.Repo != "auth-service" {
		t.Fatalf("match.Repo = %q, want auth-service", match.Repo)
	}
	if match.File != "src/auth.go" {
		t.Fatalf("match.File = %q, want src/auth.go", match.File)
	}
	if match.Line != 1 {
		t.Fatalf("match.Line = %d, want 1", match.Line)
	}
}

func TestGrepWorkspaceIncludesContextLines(t *testing.T) {
	repoRoot := t.TempDir()
	writeGrepFile(t, filepath.Join(repoRoot, "src", "client.ts"), "one\nbefore target\ntarget call\nafter call\nfive\n")

	matches, err := GrepWorkspace([]GrepRepo{
		{Name: "payments-api", Root: repoRoot},
	}, "target", GrepOptions{ContextLines: 1})
	if err != nil {
		t.Fatalf("GrepWorkspace() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}

	first := matches[0]
	if first.Line != 2 {
		t.Fatalf("first.Line = %d, want 2", first.Line)
	}
	if len(first.Before) != 1 || first.Before[0] != "one" {
		t.Fatalf("first.Before = %#v, want [one]", first.Before)
	}
	if len(first.After) != 1 || first.After[0] != "target call" {
		t.Fatalf("first.After = %#v, want [target call]", first.After)
	}

	second := matches[1]
	if second.Line != 3 {
		t.Fatalf("second.Line = %d, want 3", second.Line)
	}
	if len(second.Before) != 1 || second.Before[0] != "before target" {
		t.Fatalf("second.Before = %#v, want [before target]", second.Before)
	}
	if len(second.After) != 1 || second.After[0] != "after call" {
		t.Fatalf("second.After = %#v, want [after call]", second.After)
	}
}

func TestGrepWorkspaceSkipsBinaryFiles(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "bin", "app.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	matches, err := GrepWorkspace([]GrepRepo{
		{Name: "binary-repo", Root: repoRoot},
	}, "a", GrepOptions{})
	if err != nil {
		t.Fatalf("GrepWorkspace() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0", len(matches))
	}
}

func writeGrepFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
