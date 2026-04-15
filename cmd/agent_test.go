package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolf-jonathan/workspace-x/cmd"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestAgentInitWritesWorkspaceInstructionFiles(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "CLAUDE.md"), "# Auth Claude\nRun auth tests first.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "AGENTS.md"), "# Auth Agents\nKeep handlers thin.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", ".github", "copilot-instructions.md"), "# Copilot\nPrefer small changes.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "docs", "AGENTS.md"), "# Docs Agents\nUse the docs policy.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "docs", "nested", "CLAUDE.md"), "# Nested Claude\nAvoid broad rewrites.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "frontend", "package.json"), "{\n  \"dependencies\": {\"next\": \"1.0.0\"}\n}\n")
	writeAgentWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"agent-init", "--purpose", "Debug payment incidents"})
	command.SetOut(stdout)
	command.SetErr(stderr)

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var generatedContent string
	for index, relativePath := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(root, relativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", relativePath, err)
		}

		content := string(data)
		if index == 0 {
			generatedContent = content
		} else if content != generatedContent {
			t.Fatalf("%s content = %q, want identical content to CLAUDE.md", relativePath, content)
		}

		for _, snippet := range []string{
			"# Workspace Instructions",
			"Purpose: Debug payment incidents",
			"## Repo Instruction References",
			"### Repo: `auth-service`",
			"Paths are relative to the workspace root.",
			"- `auth-service/.github/copilot-instructions.md`",
			"- `auth-service/AGENTS.md`",
			"- `auth-service/CLAUDE.md`",
			"- `auth-service/docs/AGENTS.md`",
			"### Repo: `frontend`",
			"No repo-specific instruction files were found for this repo.",
		} {
			if !strings.Contains(content, snippet) {
				t.Fatalf("%s content = %q, want substring %q", relativePath, content, snippet)
			}
		}
	}

	for _, forbidden := range []string{
		"Run auth tests first.",
		"Keep handlers thin.",
		"Prefer small changes.",
		"Use the docs policy.",
		"Avoid broad rewrites.",
		"docs/nested/CLAUDE.md",
		"#### Source:",
	} {
		if strings.Contains(generatedContent, forbidden) {
			t.Fatalf("generated content = %q, should not contain %q", generatedContent, forbidden)
		}
	}

	output := stdout.String()
	for _, snippet := range []string{"CLAUDE.md", "AGENTS.md"} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("command output = %q, want substring %q", output, snippet)
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("command stderr = %q, want empty", stderr.String())
	}
}

func TestAgentInitOverwritesExistingWorkspaceInstructionFiles(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writeAgentWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
		},
	})
	writeAgentCmdFile(t, filepath.Join(root, "CLAUDE.md"), "user-owned\n")
	writeAgentCmdFile(t, filepath.Join(root, "AGENTS.md"), "user-owned-agents\n")

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"agent-init"})
	command.SetOut(stdout)
	command.SetErr(stderr)

	err := cmd.ExecuteCommand(command)
	if err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	for _, relativePath := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(root, relativePath)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile(%q) error = %v", relativePath, readErr)
		}
		content := string(data)
		if !strings.Contains(content, "# Workspace Instructions") {
			t.Fatalf("%s content = %q, want generated workspace instructions", relativePath, content)
		}
	}

	if !strings.Contains(stderr.String(), "Warning: overwrote existing CLAUDE.md, AGENTS.md") {
		t.Fatalf("stderr = %q, want overwrite warning", stderr.String())
	}
}

func writeAgentWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func writeAgentCmdFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
