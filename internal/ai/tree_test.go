package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderWorkspaceTreeRespectsIgnoresAndDefaultNoiseExcludes(t *testing.T) {
	repoRoot := t.TempDir()
	writeTreeFile(t, filepath.Join(repoRoot, ".gitignore"), "ignored.txt\n")
	writeTreeFile(t, filepath.Join(repoRoot, "src", "main.go"), "package main\n")
	writeTreeFile(t, filepath.Join(repoRoot, "ignored.txt"), "secret\n")
	writeTreeFile(t, filepath.Join(repoRoot, "dist", "bundle.js"), "compiled\n")
	writeTreeFile(t, filepath.Join(repoRoot, "node_modules", "left-pad", "index.js"), "module.exports = 0\n")

	output, err := RenderWorkspaceTree("payments-debug", []TreeRepo{
		{Name: "auth-service", Root: repoRoot},
	}, TreeOptions{})
	if err != nil {
		t.Fatalf("RenderWorkspaceTree() error = %v", err)
	}

	for _, snippet := range []string{
		"payments-debug/",
		"auth-service/",
		"src/",
		"main.go",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("tree output = %q, want substring %q", output, snippet)
		}
	}

	for _, unwanted := range []string{
		"ignored.txt",
		"dist/",
		"node_modules/",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("tree output = %q, want %q excluded", output, unwanted)
		}
	}
}

func TestRenderWorkspaceTreeIncludesIgnoredContentWhenRequested(t *testing.T) {
	repoRoot := t.TempDir()
	writeTreeFile(t, filepath.Join(repoRoot, ".gitignore"), "ignored.txt\n")
	writeTreeFile(t, filepath.Join(repoRoot, "ignored.txt"), "secret\n")
	writeTreeFile(t, filepath.Join(repoRoot, "node_modules", "left-pad", "index.js"), "module.exports = 0\n")

	output, err := RenderWorkspaceTree("payments-debug", []TreeRepo{
		{Name: "frontend", Root: repoRoot},
	}, TreeOptions{IncludeIgnored: true})
	if err != nil {
		t.Fatalf("RenderWorkspaceTree() error = %v", err)
	}

	for _, snippet := range []string{
		"ignored.txt",
		"node_modules/",
		"left-pad/",
		"index.js",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("tree output = %q, want substring %q", output, snippet)
		}
	}
}

func TestRenderWorkspaceTreeHonorsDepthLimit(t *testing.T) {
	repoRoot := t.TempDir()
	writeTreeFile(t, filepath.Join(repoRoot, "src", "api", "handler.go"), "package api\n")

	output, err := RenderWorkspaceTree("payments-debug", []TreeRepo{
		{Name: "service", Root: repoRoot},
	}, TreeOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("RenderWorkspaceTree() error = %v", err)
	}

	if !strings.Contains(output, "service/") {
		t.Fatalf("tree output = %q, want repo root", output)
	}
	if !strings.Contains(output, "src/") {
		t.Fatalf("tree output = %q, want first-level child", output)
	}
	if !strings.Contains(output, "...") {
		t.Fatalf("tree output = %q, want truncation marker", output)
	}
	for _, unwanted := range []string{"api/", "handler.go"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("tree output = %q, want %q omitted by depth limit", output, unwanted)
		}
	}
}

func writeTreeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
