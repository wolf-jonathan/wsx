package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jwolf/wsx/cmd"
)

func TestTreeShowsWorkspaceStructureWithDefaultIgnores(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	mustInitWorkspace(t, root, "payments-debug")

	repoRoot := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("MkdirAll(src) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ignored.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignored.txt) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "node_modules", "react"), 0o755); err != nil {
		t.Fatalf("MkdirAll(node_modules) error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", repoRoot})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"tree"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("tree ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
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
	for _, unwanted := range []string{"ignored.txt", "node_modules/"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("tree output = %q, want %q excluded", output, unwanted)
		}
	}
}

func TestTreeAllAndDepthFlagsChangeTraversal(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	mustInitWorkspace(t, root, "payments-debug")

	repoRoot := filepath.Join(t.TempDir(), "frontend")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(repoRoot) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "ignored.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignored.txt) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "src", "components"), 0o755); err != nil {
		t.Fatalf("MkdirAll(src/components) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "components", "App.tsx"), []byte("export function App() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(App.tsx) error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", repoRoot})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"tree", "--all", "--depth", "1"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("tree ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ignored.txt") {
		t.Fatalf("tree output = %q, want ignored.txt when --all is set", output)
	}
	if !strings.Contains(output, "src/") {
		t.Fatalf("tree output = %q, want src/", output)
	}
	for _, unwanted := range []string{"components/", "App.tsx"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("tree output = %q, want %q omitted by --depth 1", output, unwanted)
		}
	}
}

func TestTreeUsesBoundedDefaultDepth(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	mustInitWorkspace(t, root, "payments-debug")

	repoRoot := filepath.Join(t.TempDir(), "backend")
	if err := os.MkdirAll(filepath.Join(repoRoot, "src", "api", "handlers"), 0o755); err != nil {
		t.Fatalf("MkdirAll(src/api/handlers) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "api", "handlers", "user.go"), []byte("package handlers\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(user.go) error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", repoRoot})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"tree"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("tree ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{"src/", "api/", "..."} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("tree output = %q, want substring %q", output, snippet)
		}
	}
	for _, unwanted := range []string{"handlers/", "user.go"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("tree output = %q, want %q omitted by default depth", output, unwanted)
		}
	}
}
