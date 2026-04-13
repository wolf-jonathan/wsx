package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jwolf/wsx/cmd"
	"github.com/jwolf/wsx/internal/workspace"
)

func TestListShowsLiveStatusAndRuntimeLinkType(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	healthyTarget := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(healthyTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll(healthyTarget) error = %v", err)
	}

	brokenTarget := filepath.Join(t.TempDir(), "old-service")
	if err := os.MkdirAll(brokenTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll(brokenTarget) error = %v", err)
	}

	addHealthy := cmd.NewRootCommand()
	addHealthy.SetArgs([]string{"add", healthyTarget})
	addHealthy.SetOut(new(bytes.Buffer))
	addHealthy.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(addHealthy); err != nil {
		t.Fatalf("add healthy ExecuteCommand() error = %v", err)
	}

	addBroken := cmd.NewRootCommand()
	addBroken.SetArgs([]string{"add", brokenTarget})
	addBroken.SetOut(new(bytes.Buffer))
	addBroken.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(addBroken); err != nil {
		t.Fatalf("add broken ExecuteCommand() error = %v", err)
	}

	if err := os.RemoveAll(brokenTarget); err != nil {
		t.Fatalf("RemoveAll(brokenTarget) error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"list"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("list ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		"NAME",
		"PATH",
		"TYPE",
		"STATUS",
		"auth-service",
		"old-service",
		"ok",
		"broken",
		workspace.LinkTypeSymlink,
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("list output = %q, want substring %q", output, snippet)
		}
	}
}

func TestListJSONIncludesResolvedPathStatusAndLinkType(t *testing.T) {
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

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"add", `${WORK_REPOS}\payments-api`})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("add ExecuteCommand() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	list := cmd.NewRootCommand()
	list.SetArgs([]string{"list", "--json"})
	list.SetOut(stdout)
	list.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(list); err != nil {
		t.Fatalf("list ExecuteCommand() error = %v", err)
	}

	var items []struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		ResolvedPath string `json:"resolved_path"`
		LinkType     string `json:"link_type"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	item := items[0]
	if item.Name != "payments-api" {
		t.Fatalf("item.Name = %q, want payments-api", item.Name)
	}
	if item.Path != "${WORK_REPOS}/payments-api" {
		t.Fatalf("item.Path = %q, want ${WORK_REPOS}/payments-api", item.Path)
	}
	if item.ResolvedPath != target {
		t.Fatalf("item.ResolvedPath = %q, want %q", item.ResolvedPath, target)
	}
	if item.LinkType != workspace.LinkTypeSymlink {
		t.Fatalf("item.LinkType = %q, want %q", item.LinkType, workspace.LinkTypeSymlink)
	}
	if item.Status != "ok" {
		t.Fatalf("item.Status = %q, want ok", item.Status)
	}
}

func TestListMarksRepointedWorkspaceLinkBroken(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	mustInitWorkspace(t, root, "payments-debug")

	reposRoot := filepath.Join(t.TempDir(), "repos")
	configuredTarget := filepath.Join(reposRoot, "payments-api")
	actualTarget := filepath.Join(reposRoot, "wrong-api")
	for _, dir := range []string{configuredTarget, actualTarget} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), []byte("WORK_REPOS="+reposRoot+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}
	if _, err := workspace.CreateLink(actualTarget, filepath.Join(root, "payments-api")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if err := workspace.SaveConfig(root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "payments-api", Path: `${WORK_REPOS}/payments-api`},
		},
	}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	list := cmd.NewRootCommand()
	list.SetArgs([]string{"list", "--json"})
	list.SetOut(stdout)
	list.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(list); err != nil {
		t.Fatalf("list ExecuteCommand() error = %v", err)
	}

	var items []struct {
		Name         string `json:"name"`
		ResolvedPath string `json:"resolved_path"`
		LinkType     string `json:"link_type"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ResolvedPath != configuredTarget {
		t.Fatalf("item.ResolvedPath = %q, want %q", items[0].ResolvedPath, configuredTarget)
	}
	if items[0].Status != "broken" {
		t.Fatalf("item.Status = %q, want broken", items[0].Status)
	}
	if items[0].LinkType != "" {
		t.Fatalf("item.LinkType = %q, want empty for repointed broken link", items[0].LinkType)
	}
}
