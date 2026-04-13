package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpWorkspaceRespectsIgnorePathAndGlobFilters(t *testing.T) {
	repoRoot := t.TempDir()
	writeDumpFile(t, filepath.Join(repoRoot, ".gitignore"), "src/api/private.txt\n")
	writeDumpFile(t, filepath.Join(repoRoot, "src", "api", "openapi.yaml"), "openapi: 3.0.0\n")
	writeDumpFile(t, filepath.Join(repoRoot, "src", "api", "notes.txt"), "notes\n")
	writeDumpFile(t, filepath.Join(repoRoot, "src", "api", "private.txt"), "secret\n")
	writeDumpFile(t, filepath.Join(repoRoot, "src", "api", "package.lock"), "lock\n")

	result, err := DumpWorkspace("payments-debug", []DumpRepo{
		{Name: "payments-api", Root: repoRoot},
	}, DumpOptions{
		IncludeGlobs: []string{"*.yaml", "*.txt", "*.lock"},
		ExcludeGlobs: []string{"*.lock", "notes.txt"},
		PathPrefix:   "src/api",
	})
	if err != nil {
		t.Fatalf("DumpWorkspace() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("len(result.Files) = %d, want 1", len(result.Files))
	}
	if result.Files[0].Repo != "payments-api" {
		t.Fatalf("result.Files[0].Repo = %q, want payments-api", result.Files[0].Repo)
	}
	if result.Files[0].File != "src/api/openapi.yaml" {
		t.Fatalf("result.Files[0].File = %q, want src/api/openapi.yaml", result.Files[0].File)
	}
	if result.Files[0].Content != "openapi: 3.0.0\n" {
		t.Fatalf("result.Files[0].Content = %q, want openapi content", result.Files[0].Content)
	}
}

func TestDumpWorkspaceCanIncludeIgnoredFilesButStillSkipsBuiltInNoise(t *testing.T) {
	repoRoot := t.TempDir()
	writeDumpFile(t, filepath.Join(repoRoot, ".gitignore"), "ignored.txt\n")
	writeDumpFile(t, filepath.Join(repoRoot, "ignored.txt"), "secret\n")
	writeDumpFile(t, filepath.Join(repoRoot, ".git", "config"), "[core]\n")

	result, err := DumpWorkspace("payments-debug", []DumpRepo{
		{Name: "auth-service", Root: repoRoot},
	}, DumpOptions{
		AllFiles:       true,
		IncludeIgnored: true,
	})
	if err != nil {
		t.Fatalf("DumpWorkspace() error = %v", err)
	}

	if len(result.Files) != 2 {
		t.Fatalf("len(result.Files) = %d, want 2", len(result.Files))
	}
	if result.Files[0].File != ".gitignore" {
		t.Fatalf("result.Files[0].File = %q, want .gitignore", result.Files[0].File)
	}
	if result.Files[1].File != "ignored.txt" {
		t.Fatalf("result.Files[1].File = %q, want ignored.txt", result.Files[1].File)
	}
}

func TestDumpWorkspaceSupportsDryRunAndTokenTruncation(t *testing.T) {
	repoRoot := t.TempDir()
	writeDumpFile(t, filepath.Join(repoRoot, "a.txt"), "abcdefgh\n")
	writeDumpFile(t, filepath.Join(repoRoot, "b.txt"), "ijklmnop\n")

	dryRun, err := DumpWorkspace("payments-debug", []DumpRepo{
		{Name: "frontend", Root: repoRoot},
	}, DumpOptions{
		AllFiles: true,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("DumpWorkspace() dry run error = %v", err)
	}

	if len(dryRun.Files) != 2 {
		t.Fatalf("len(dryRun.Files) = %d, want 2", len(dryRun.Files))
	}
	if dryRun.Files[0].Content != "" || dryRun.Files[1].Content != "" {
		t.Fatalf("dryRun.Files contents = %#v, want empty content", dryRun.Files)
	}

	truncated, err := DumpWorkspace("payments-debug", []DumpRepo{
		{Name: "frontend", Root: repoRoot},
	}, DumpOptions{
		AllFiles:  true,
		MaxTokens: 3,
	})
	if err != nil {
		t.Fatalf("DumpWorkspace() truncated error = %v", err)
	}

	if !truncated.Truncated {
		t.Fatal("truncated.Truncated = false, want true")
	}
	if len(truncated.Files) != 1 {
		t.Fatalf("len(truncated.Files) = %d, want 1", len(truncated.Files))
	}
	if truncated.Files[0].File != "a.txt" {
		t.Fatalf("truncated.Files[0].File = %q, want a.txt", truncated.Files[0].File)
	}
}

func TestRenderDumpMarkdownIncludesHeadersCodeFencesAndWarning(t *testing.T) {
	output := RenderDumpMarkdown(DumpResult{
		Workspace: "payments-debug",
		Files: []DumpFile{
			{Repo: "auth-service", File: "src/main.go", Content: "package main\n"},
		},
		Truncated: true,
	}, false)

	for _, snippet := range []string{
		"# Workspace: payments-debug",
		"Warning: output truncated",
		"## auth-service/src/main.go",
		"```go",
		"package main",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("RenderDumpMarkdown() = %q, want substring %q", output, snippet)
		}
	}
}

func TestValidateDumpScopeRequiresNarrowingFilter(t *testing.T) {
	err := ValidateDumpScope(DumpOptions{})
	if err == nil {
		t.Fatal("ValidateDumpScope() error = nil, want narrowing requirement")
	}

	if !strings.Contains(err.Error(), "wsx dump requires a filter") {
		t.Fatalf("ValidateDumpScope() error = %q, want filter guidance", err.Error())
	}
}

func writeDumpFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
