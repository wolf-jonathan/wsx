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
	writeAgentEnvFile(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"agent-init", "--purpose", "Debug payment incidents"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	for _, relativePath := range []string{"CLAUDE.md", "AGENTS.md"} {
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
		} {
			if !strings.Contains(content, snippet) {
				t.Fatalf("%s content = %q, want substring %q", relativePath, content, snippet)
			}
		}
	}

	output := stdout.String()
	for _, snippet := range []string{"CLAUDE.md", "AGENTS.md"} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("command output = %q, want substring %q", output, snippet)
		}
	}
}

func TestAgentInitFailsWhenWorkspaceInstructionFileAlreadyExists(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	writeAgentWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeAgentCmdFile(t, filepath.Join(reposRoot, "auth-service", "go.mod"), "module example.com/auth\n")
	writeAgentEnvFile(t, root, reposRoot)
	writeAgentCmdFile(t, filepath.Join(root, "CLAUDE.md"), "user-owned\n")

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"agent-init"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want existing file error")
	}

	if !strings.Contains(err.Error(), "CLAUDE.md already exists") {
		t.Fatalf("error = %q, want existing file message", err.Error())
	}

	if _, statErr := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Fatalf("AGENTS.md should not be created when agent-init fails, stat error = %v", statErr)
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
