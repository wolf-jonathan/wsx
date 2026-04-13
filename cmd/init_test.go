package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolf-jonathan/workspace-x/cmd"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestInitCreatesWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"init", "payments-debug"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, workspace.ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile(.wsx.json) error = %v", err)
	}

	var cfg workspace.Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if cfg.Version != "1" {
		t.Fatalf("Version = %q, want 1", cfg.Version)
	}

	if cfg.Name != "payments-debug" {
		t.Fatalf("Name = %q, want payments-debug", cfg.Name)
	}

	if len(cfg.Refs) != 0 {
		t.Fatalf("Refs length = %d, want 0", len(cfg.Refs))
	}

	envContent, err := os.ReadFile(filepath.Join(root, workspace.EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.wsx.env) error = %v", err)
	}

	if string(envContent) != "" {
		t.Fatalf(".wsx.env content = %q, want empty file", string(envContent))
	}

	gitignoreContent, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) error = %v", err)
	}

	if string(gitignoreContent) != workspace.EnvFileName+"\n" {
		t.Fatalf(".gitignore content = %q, want %q", string(gitignoreContent), workspace.EnvFileName+"\\n")
	}
}

func TestInitDefaultsNameAndAppendsGitignoreOnce(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.gitignore) error = %v", err)
	}

	chdirForTest(t, root)

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"init"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	loaded, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Config.Name != "checkout" {
		t.Fatalf("Name = %q, want checkout", loaded.Config.Name)
	}

	gitignoreContent, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(gitignoreContent)), "\n")
	if len(lines) != 2 {
		t.Fatalf(".gitignore lines = %#v, want 2 lines", lines)
	}

	if lines[0] != "node_modules/" {
		t.Fatalf("first .gitignore line = %q, want node_modules/", lines[0])
	}

	if lines[1] != workspace.EnvFileName {
		t.Fatalf("second .gitignore line = %q, want %q", lines[1], workspace.EnvFileName)
	}
}

func TestInitFailsWhenWorkspaceAlreadyInitialized(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	original := workspace.Config{
		Version: "1",
		Name:    "existing-workspace",
	}
	if err := workspace.SaveConfig(root, original); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"init", "new-name"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	stdout := new(bytes.Buffer)
	command.SetOut(stdout)

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("Execute() error = nil, want already-initialized error")
	}

	if !strings.Contains(err.Error(), workspace.ConfigFileName) {
		t.Fatalf("error = %q, want mention of %s", err.Error(), workspace.ConfigFileName)
	}

	if strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want no help output for runtime error", stdout.String())
	}

	loaded, loadErr := workspace.LoadConfig(root)
	if loadErr != nil {
		t.Fatalf("LoadConfig() error = %v", loadErr)
	}

	if loaded.Config.Name != "existing-workspace" {
		t.Fatalf("Name after failed init = %q, want existing-workspace", loaded.Config.Name)
	}

	if _, statErr := os.Stat(filepath.Join(root, workspace.EnvFileName)); !os.IsNotExist(statErr) {
		t.Fatalf(".wsx.env stat error = %v, want not exists", statErr)
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	})
}
