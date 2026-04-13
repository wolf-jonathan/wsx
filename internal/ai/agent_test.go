package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWorkspaceInstructionsIncludesRepoSpecificInstructionFiles(t *testing.T) {
	authRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(authRoot, "go.mod"), "module example.com/auth\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "CLAUDE.md"), "# Auth Claude\nUse go test ./...\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "docs", "AGENTS.md"), "# Auth Agents\nUse the auth repo policy.\n")

	frontendRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(frontendRoot, "package.json"), "{\n  \"dependencies\": {\"react\": \"1.0.0\"}\n}\n")

	instructions, err := GenerateWorkspaceInstructions("payments-debug", "Debug payment incidents", []InstructionRepo{
		{Name: "auth-service", Root: authRoot},
		{Name: "frontend", Root: frontendRoot},
	})
	if err != nil {
		t.Fatalf("GenerateWorkspaceInstructions() error = %v", err)
	}

	content := RenderWorkspaceInstructions(instructions)

	for _, snippet := range []string{
		"# Workspace Instructions",
		"Purpose: Debug payment incidents",
		"`auth-service` (" + filepath.ToSlash(authRoot) + ") - Go",
		"`frontend` (" + filepath.ToSlash(frontendRoot) + ") - Node.js / React",
		"### Repo: `auth-service`",
		"#### Source: `CLAUDE.md`",
		"#### Source: `docs/AGENTS.md`",
		"This section applies when working in linked repo `auth-service`.",
		"# Auth Claude",
		"# Auth Agents",
		"### Repo: `frontend`",
		"No repo-specific instruction files were found for this repo.",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("instructions content = %q, want substring %q", content, snippet)
		}
	}
}

func TestWriteWorkspaceInstructionFilesWritesAllTargets(t *testing.T) {
	root := t.TempDir()
	content := "# Workspace Instructions\n"

	if err := WriteWorkspaceInstructionFiles(root, content); err != nil {
		t.Fatalf("WriteWorkspaceInstructionFiles() error = %v", err)
	}

	for _, relativePath := range []string{
		WorkspaceClaudeFilePath,
		WorkspaceAgentsFilePath,
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", relativePath, err)
		}
		if string(data) != content {
			t.Fatalf("file %q = %q, want %q", relativePath, string(data), content)
		}
	}
}

func TestWriteWorkspaceInstructionFilesFailsWhenTargetAlreadyExists(t *testing.T) {
	root := t.TempDir()
	content := "# Workspace Instructions\n"

	writeAgentTestFile(t, filepath.Join(root, WorkspaceClaudeFilePath), "user content\n")

	err := WriteWorkspaceInstructionFiles(root, content)
	if err == nil {
		t.Fatal("WriteWorkspaceInstructionFiles() error = nil, want existing file error")
	}

	if !strings.Contains(err.Error(), WorkspaceClaudeFilePath) {
		t.Fatalf("error = %q, want mention of %q", err.Error(), WorkspaceClaudeFilePath)
	}

	if _, statErr := os.Stat(filepath.Join(root, WorkspaceAgentsFilePath)); !os.IsNotExist(statErr) {
		t.Fatalf("%s should not be created when preflight fails, stat error = %v", WorkspaceAgentsFilePath, statErr)
	}
}

func writeAgentTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
