package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWorkspaceInstructionsIncludesInstructionReferences(t *testing.T) {
	authRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(authRoot, "go.mod"), "module example.com/auth\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "CLAUDE.md"), "# Auth Claude\nUse go test ./...\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "AGENTS.md"), "# Root Auth Agents\nUse the root policy.\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "docs", "AGENTS.md"), "# Docs Agents\nUse the auth repo policy.\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "docs", "nested", "CLAUDE.md"), "# Nested Claude\nKeep handlers thin.\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "services", "CLAUDE.md"), "# Services Claude\nPrefer service contracts.\n")
	writeAgentTestFile(t, filepath.Join(authRoot, ".github", "copilot-instructions.md"), "# Copilot Instructions\nPrefer small changes.\n")
	writeAgentTestFile(t, filepath.Join(authRoot, "docs", "readme.md"), "ignored content\n")

	frontendRoot := t.TempDir()
	writeAgentTestFile(t, filepath.Join(frontendRoot, "package.json"), "{\n  \"dependencies\": {\"react\": \"1.0.0\"}\n}\n")

	instructions, err := GenerateWorkspaceInstructions("payments-debug", "Debug payment incidents", []InstructionRepo{
		{Name: "auth-service", Root: authRoot},
		{Name: "frontend", Root: frontendRoot},
	})
	if err != nil {
		t.Fatalf("GenerateWorkspaceInstructions() error = %v", err)
	}

	if got, want := len(instructions.Repos), 2; got != want {
		t.Fatalf("len(instructions.Repos) = %d, want %d", got, want)
	}

	authRepo := instructions.Repos[0]
	wantReferences := []string{
		"auth-service/.github/copilot-instructions.md",
		"auth-service/AGENTS.md",
		"auth-service/CLAUDE.md",
		"auth-service/docs/AGENTS.md",
		"auth-service/services/CLAUDE.md",
	}
	if got := referencePaths(authRepo.References); !slicesEqual(got, wantReferences) {
		t.Fatalf("auth repo references = %v, want %v", got, wantReferences)
	}

	frontendRepo := instructions.Repos[1]
	if len(frontendRepo.References) != 0 {
		t.Fatalf("frontend repo references = %v, want none", referencePaths(frontendRepo.References))
	}

	content := RenderWorkspaceInstructions(instructions)

	for _, snippet := range []string{
		"# Workspace Instructions",
		"Purpose: Debug payment incidents",
		"`auth-service` (" + filepath.ToSlash(authRoot) + ") - Go",
		"`frontend` (" + filepath.ToSlash(frontendRoot) + ") - Node.js / React",
		"## Repo Instruction References",
		"### Repo: `auth-service`",
		"This section lists instruction file references only. Contents are not duplicated here.",
		"Paths are relative to the workspace root.",
		"- `auth-service/.github/copilot-instructions.md`",
		"- `auth-service/AGENTS.md`",
		"- `auth-service/CLAUDE.md`",
		"- `auth-service/docs/AGENTS.md`",
		"- `auth-service/services/CLAUDE.md`",
		"### Repo: `frontend`",
		"No repo-specific instruction files were found for this repo.",
	} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("instructions content = %q, want substring %q", content, snippet)
		}
	}

	for _, forbidden := range []string{
		"Use go test ./...",
		"Use the root policy.",
		"Use the auth repo policy.",
		"Keep handlers thin.",
		"Prefer service contracts.",
		"Prefer small changes.",
		"- `.github/copilot-instructions.md`",
		"- `AGENTS.md`",
		"- `CLAUDE.md`",
		"- `docs/AGENTS.md`",
		"docs/nested/CLAUDE.md",
		"#### Source:",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("instructions content = %q, should not contain %q", content, forbidden)
		}
	}
}

func TestBuildWorkspaceInstructionContentMatchesRenderedInstructions(t *testing.T) {
	root := t.TempDir()
	writeAgentTestFile(t, filepath.Join(root, "go.mod"), "module example.com/auth\n")
	writeAgentTestFile(t, filepath.Join(root, "AGENTS.md"), "# Root Auth Agents\n")

	repos := []InstructionRepo{{Name: "auth-service", Root: root}}

	built, err := BuildWorkspaceInstructionContent("payments-debug", "Debug payment incidents", repos)
	if err != nil {
		t.Fatalf("BuildWorkspaceInstructionContent() error = %v", err)
	}

	instructions, err := GenerateWorkspaceInstructions("payments-debug", "Debug payment incidents", repos)
	if err != nil {
		t.Fatalf("GenerateWorkspaceInstructions() error = %v", err)
	}

	rendered := RenderWorkspaceInstructions(instructions)
	if built != rendered {
		t.Fatalf("BuildWorkspaceInstructionContent() = %q, want %q", built, rendered)
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

func referencePaths(references []InstructionReference) []string {
	paths := make([]string, 0, len(references))
	for _, reference := range references {
		paths = append(paths, reference.Path)
	}
	return paths
}

func slicesEqual[T comparable](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
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
