package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jwolf/wsx/cmd"
	"github.com/jwolf/wsx/internal/workspace"
)

func TestAgentInitWritesWorkspaceInstructionFiles(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	writeAgentWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
			{Name: "frontend", Path: `${WORK_REPOS}/frontend`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "CLAUDE.md"), "# Auth Claude\nRun auth tests first.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "AGENTS.md"), "# Auth Agents\nKeep handlers thin.\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "frontend", "package.json"), "{\n  \"dependencies\": {\"next\": \"1.0.0\"}\n}\n")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "frontend", ".github", "copilot-instructions.md"), "# Frontend Copilot\nPrefer server components.\n")
	writeAgentEnvFile(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"agent-init", "--purpose", "Debug payment incidents"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	for _, relativePath := range []string{"CLAUDE.md", "AGENTS.md", filepath.Join(".github", "copilot-instructions.md")} {
		path := filepath.Join(root, relativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", relativePath, err)
		}

		content := string(data)
		for _, snippet := range []string{
			"# Workspace Instructions",
			"Purpose: Debug payment incidents",
			"### Repo: `auth-service`",
			"#### Source: `CLAUDE.md`",
			"#### Source: `AGENTS.md`",
			"# Auth Claude",
			"# Auth Agents",
			"### Repo: `frontend`",
			"#### Source: `.github/copilot-instructions.md`",
			"# Frontend Copilot",
		} {
			if !strings.Contains(content, snippet) {
				t.Fatalf("%s content = %q, want substring %q", relativePath, content, snippet)
			}
		}
	}

	output := stdout.String()
	for _, snippet := range []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("command output = %q, want substring %q", output, snippet)
		}
	}
}

func writeAgentWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func writeAgentEnvFile(t *testing.T, root, reposRoot string) {
	t.Helper()

	content := []byte("WORK_REPOS=" + reposRoot + "\n")
	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), content, 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
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
