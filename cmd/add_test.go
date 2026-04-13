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

func TestAddCreatesLinkAndStoresParameterizedPath(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), []byte("WORK_REPOS="+reposRoot+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"add", target})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	loaded, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(loaded.Config.Refs) != 1 {
		t.Fatalf("refs length = %d, want 1", len(loaded.Config.Refs))
	}

	ref := loaded.Config.Refs[0]
	if ref.Name != "auth-service" {
		t.Fatalf("ref.Name = %q, want auth-service", ref.Name)
	}

	if ref.Path != "${WORK_REPOS}/auth-service" {
		t.Fatalf("ref.Path = %q, want ${WORK_REPOS}/auth-service", ref.Path)
	}

	linkPath := filepath.Join(root, "auth-service")
	if _, err := workspace.DetectLinkType(linkPath); err != nil {
		t.Fatalf("DetectLinkType(%q) error = %v", linkPath, err)
	}

	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", linkPath, err)
	}

	if !sameDirectoryTarget(t, resolved, target) {
		t.Fatalf("resolved link target = %q, want same target as %q", resolved, target)
	}
}

func TestAddSupportsParameterizedInputAndCustomName(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "payments-api")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), []byte("WORK_REPOS="+reposRoot+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"add", `${WORK_REPOS}\payments-api`, "--as", "payments"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	loaded, err := workspace.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	ref := loaded.Config.Refs[0]
	if ref.Name != "payments" {
		t.Fatalf("ref.Name = %q, want payments", ref.Name)
	}

	if ref.Path != "${WORK_REPOS}/payments-api" {
		t.Fatalf("ref.Path = %q, want ${WORK_REPOS}/payments-api", ref.Path)
	}

	linkPath := filepath.Join(root, "payments")
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", linkPath, err)
	}

	if !sameDirectoryTarget(t, resolved, target) {
		t.Fatalf("resolved link target = %q, want same target as %q", resolved, target)
	}
}

func TestAddRejectsNameConflictsWithoutMutatingConfig(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	cfg := workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{
				Name: "auth-service",
				Path: "C:/repos/auth-service",
			},
		},
	}
	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), nil, 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"add", target})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want name conflict error")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("error = %q, want conflict message", err.Error())
	}

	loaded, loadErr := workspace.LoadConfig(root)
	if loadErr != nil {
		t.Fatalf("LoadConfig() error = %v", loadErr)
	}

	if len(loaded.Config.Refs) != 1 {
		t.Fatalf("refs length after failure = %d, want 1", len(loaded.Config.Refs))
	}

	if _, statErr := os.Lstat(filepath.Join(root, "auth-service")); !os.IsNotExist(statErr) {
		t.Fatalf("workspace link stat error = %v, want not exists", statErr)
	}
}

func TestAddRejectsCircularReferencesWithoutMutatingConfig(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"add", root})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want circular reference error")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "circular") {
		t.Fatalf("error = %q, want circular reference message", err.Error())
	}

	loaded, loadErr := workspace.LoadConfig(root)
	if loadErr != nil {
		t.Fatalf("LoadConfig() error = %v", loadErr)
	}

	if len(loaded.Config.Refs) != 0 {
		t.Fatalf("refs length after failure = %d, want 0", len(loaded.Config.Refs))
	}
}

func mustInitWorkspace(t *testing.T, root, name string) {
	t.Helper()

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"init", name})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("init ExecuteCommand() error = %v", err)
	}
}

func sameDirectoryTarget(t *testing.T, left, right string) bool {
	t.Helper()

	leftInfo, err := os.Stat(left)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", left, err)
	}

	rightInfo, err := os.Stat(right)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", right, err)
	}

	return os.SameFile(leftInfo, rightInfo)
}
