package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jwolf/wsx/internal/workspace"
)

func TestPromptRendersWorkspaceSummaryTreeAndDetections(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)
	writeExecWorkspace(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
			{Name: "frontend", Path: `${WORK_REPOS}/frontend`},
			{Name: "misc", Path: `${WORK_REPOS}/misc`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writePromptFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writePromptFile(t, filepath.Join(reposRoot, "frontend", "package.json"), "{\n  \"dependencies\": {\"next\": \"14.2.0\"}\n}\n")
	if err := os.MkdirAll(filepath.Join(reposRoot, "misc", "src"), 0o755); err != nil {
		t.Fatalf("MkdirAll(misc/src) error = %v", err)
	}
	writeExecEnv(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"prompt"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		`You are working in a multi-repo workspace called "payments-debug".`,
		"- auth-service (" + filepath.ToSlash(filepath.Join(reposRoot, "auth-service")) + ") - Go",
		"- frontend (" + filepath.ToSlash(filepath.Join(reposRoot, "frontend")) + ") - Node.js / Next.js",
		"- misc (" + filepath.ToSlash(filepath.Join(reposRoot, "misc")) + ") - Unknown",
		"These repository directories are linked into the workspace by `wsx`",
		"Directory structure:",
		"payments-debug/",
		"auth-service/",
		"frontend/",
		"misc/",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("prompt output = %q, want substring %q", output, snippet)
		}
	}
}

func TestPromptCopyWritesToClipboard(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)
	writeExecWorkspace(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writePromptFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writeExecEnv(t, root, reposRoot)

	var copied string
	restore := swapPromptClipboardWriter(func(content string) error {
		copied = content
		return nil
	})
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"prompt", "--copy"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	if copied == "" {
		t.Fatal("copied prompt = empty, want clipboard content")
	}
	if copied != stdout.String() {
		t.Fatalf("copied prompt != stdout\ncopied: %q\nstdout: %q", copied, stdout.String())
	}
}

func writePromptFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func swapPromptClipboardWriter(writer func(string) error) func() {
	previous := writePromptClipboard
	writePromptClipboard = writer
	return func() {
		writePromptClipboard = previous
	}
}
