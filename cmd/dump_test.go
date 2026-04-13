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

func TestDumpRequiresNarrowingFilterByDefault(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	mustInitWorkspace(t, root, "payments-debug")

	command := cmd.NewRootCommand()
	command.SetArgs([]string{"dump"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want filter requirement")
	}

	for _, snippet := range []string{
		"wsx dump requires a filter",
		"Use --include, --path, or --repo",
		"--all-files",
		"wsx tree",
	} {
		if !strings.Contains(err.Error(), snippet) {
			t.Fatalf("ExecuteCommand() error = %q, want substring %q", err.Error(), snippet)
		}
	}
}

func TestDumpMarkdownRespectsIgnoresAndSelectedPath(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	writeDumpWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "auth-service", ".gitignore"), "src/api/secret.txt\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "auth-service", "src", "api", "openapi.yaml"), "openapi: 3.0.0\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "auth-service", "src", "api", "secret.txt"), "secret\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "auth-service", "README.md"), "skip me\n")
	writeDumpEnvFile(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"dump", "--include", "*.yaml,*.txt", "--path", "src/api"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		"# Workspace: payments-debug",
		"## auth-service/src/api/openapi.yaml",
		"```yaml",
		"openapi: 3.0.0",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("dump output = %q, want substring %q", output, snippet)
		}
	}

	for _, unwanted := range []string{"secret.txt", "README.md"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("dump output = %q, want %q excluded", output, unwanted)
		}
	}
}

func TestDumpJSONDryRunCanBypassGitignoreForOneRepo(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	writeDumpWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "frontend", Path: `${WORK_REPOS}/frontend`},
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "frontend", ".gitignore"), "ignored.txt\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "frontend", "ignored.txt"), "secret\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "frontend", "visible.txt"), "visible\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "auth-service", "ignored.txt"), "other repo\n")
	writeDumpEnvFile(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"dump", "--repo", "frontend", "--include", "*.txt", "--no-ignore", "--dry-run", "--format", "json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var files []struct {
		Repo    string `json:"repo"`
		File    string `json:"file"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &files); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(files))
	}

	for _, file := range files {
		if file.Repo != "frontend" {
			t.Fatalf("file.Repo = %q, want frontend", file.Repo)
		}
		if file.Content != "" {
			t.Fatalf("file.Content = %q, want empty in dry-run", file.Content)
		}
	}

	paths := []string{files[0].File, files[1].File}
	if strings.Join(paths, ",") != "ignored.txt,visible.txt" {
		t.Fatalf("files = %v, want frontend ignored.txt and visible.txt in sorted order", paths)
	}
}

func TestDumpMaxTokensAddsWarningAndStopsAfterBudget(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	writeDumpWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "frontend", Path: `${WORK_REPOS}/frontend`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "frontend", "a.txt"), "abcdefgh\n")
	writeCmdDumpFile(t, filepath.Join(reposRoot, "frontend", "b.txt"), "ijklmnop\n")
	writeDumpEnvFile(t, root, reposRoot)

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"dump", "--all-files", "--max-tokens", "3"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Warning: output truncated") {
		t.Fatalf("dump output = %q, want truncation warning", output)
	}
	if !strings.Contains(output, "## frontend/a.txt") {
		t.Fatalf("dump output = %q, want first file", output)
	}
	if strings.Contains(output, "## frontend/b.txt") {
		t.Fatalf("dump output = %q, want second file omitted after token budget", output)
	}
}

func writeDumpWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func writeDumpEnvFile(t *testing.T, root, reposRoot string) {
	t.Helper()

	content := []byte("WORK_REPOS=" + reposRoot + "\n")
	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), content, 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}
}

func writeCmdDumpFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
