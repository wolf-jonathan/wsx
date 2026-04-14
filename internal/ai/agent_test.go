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
	writeAgentTestFile(t, filepath.Join(authRoot, "AGENTS.md"), "# Root Auth Agents\nUse the root policy.\n")

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
		"#### Source: `AGENTS.md`",
		"This section applies when working in linked repo `auth-service`.",
		"## Auth Claude",
		"## Root Auth Agents",
		"### Repo: `frontend`",
		"No repo-specific instruction files were found for this repo.",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("instructions content = %q, want substring %q", content, snippet)
		}
	}

	for _, forbidden := range []string{
		"\n# Auth Claude\n",
		"\n# Root Auth Agents\n",
		"docs/AGENTS.md",
		"## Auth Agents",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("instructions content = %q, should not contain %q", content, forbidden)
		}
	}
}

func TestNormalizeImportedInstructionMarkdownDemotesHeadersByOneLevel(t *testing.T) {
	content := "# Top\n## Mid\n### Low\n###### Deep\nnot a header\n#NoSpace\n  # Indented\n"

	got := normalizeImportedInstructionMarkdown(content)
	want := "## Top\n### Mid\n#### Low\n###### Deep\nnot a header\n#NoSpace\n  ## Indented\n"

	if got != want {
		t.Fatalf("normalizeImportedInstructionMarkdown() = %q, want %q", got, want)
	}
}

func TestWriteWorkspaceInstructionFilesWritesAllTargets(t *testing.T) {
	root := t.TempDir()
	content := "# Workspace Instructions\n"

	overwritten, err := WriteWorkspaceInstructionFiles(root, content)
	if err != nil {
		t.Fatalf("WriteWorkspaceInstructionFiles() error = %v", err)
	}
	if len(overwritten) != 0 {
		t.Fatalf("WriteWorkspaceInstructionFiles() overwritten = %v, want none", overwritten)
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

func TestWriteWorkspaceInstructionFilesOverwritesExistingTargets(t *testing.T) {
	root := t.TempDir()
	content := "# Workspace Instructions\n"

	writeAgentTestFile(t, filepath.Join(root, WorkspaceClaudeFilePath), "user content\n")
	writeAgentTestFile(t, filepath.Join(root, WorkspaceAgentsFilePath), "user agents content\n")

	overwritten, err := WriteWorkspaceInstructionFiles(root, content)
	if err != nil {
		t.Fatalf("WriteWorkspaceInstructionFiles() error = %v", err)
	}

	if got := strings.Join(overwritten, ", "); got != "CLAUDE.md, AGENTS.md" {
		t.Fatalf("WriteWorkspaceInstructionFiles() overwritten = %q, want %q", got, "CLAUDE.md, AGENTS.md")
	}

	for _, relativePath := range []string{WorkspaceClaudeFilePath, WorkspaceAgentsFilePath} {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", relativePath, readErr)
		}
		if string(data) != content {
			t.Fatalf("%s = %q, want %q", relativePath, string(data), content)
		}
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
