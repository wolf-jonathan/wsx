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

func TestRemoveDeletesConfigEntryAndWorkspaceLink(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	target := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", target})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	remove := cmd.NewRootCommand()
	remove.SetArgs([]string{"remove", "auth-service"})
	remove.SetOut(new(bytes.Buffer))
	remove.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(remove); err != nil {
		t.Fatalf("remove ExecuteCommand() error = %v", err)
	}

	loaded, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(loaded.Config.Refs) != 0 {
		t.Fatalf("refs length after remove = %d, want 0", len(loaded.Config.Refs))
	}

	linkPath := filepath.Join(root, "auth-service")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("workspace link stat error = %v, want not exists", err)
	}
}

func TestRemoveLeavesTargetDirectoryUntouched(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	target := filepath.Join(t.TempDir(), "payments-api")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	marker := filepath.Join(target, "README.md")
	if err := os.WriteFile(marker, []byte("repo stays"), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", target})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	remove := cmd.NewRootCommand()
	remove.SetArgs([]string{"remove", "payments-api"})
	remove.SetOut(new(bytes.Buffer))
	remove.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(remove); err != nil {
		t.Fatalf("remove ExecuteCommand() error = %v", err)
	}

	content, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("ReadFile(marker) error = %v", err)
	}

	if string(content) != "repo stays" {
		t.Fatalf("marker content = %q, want repo stays", string(content))
	}
}

func TestRemoveRejectsUnknownRefWithoutMutatingConfig(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	remove := cmd.NewRootCommand()
	remove.SetArgs([]string{"remove", "missing"})
	remove.SetOut(new(bytes.Buffer))
	remove.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(remove)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want missing ref error")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Fatalf("error = %q, want not found message", err.Error())
	}

	loaded, loadErr := workspace.LoadConfig(root)
	if loadErr != nil {
		t.Fatalf("LoadConfig() error = %v", loadErr)
	}

	if len(loaded.Config.Refs) != 0 {
		t.Fatalf("refs length after failure = %d, want 0", len(loaded.Config.Refs))
	}
}
