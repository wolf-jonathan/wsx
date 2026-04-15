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

func TestGrepFindsMatchesWithIncludeExcludeAndContext(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "auth-service", ".gitignore"), "ignored.txt\n")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "auth-service", "src", "middleware.go"), "package auth\nfunc handleAuth() {}\nreturn handleAuth(ctx)\n")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "auth-service", "ignored.txt"), "handleAuth hidden\n")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "payments-api", "src", "client.ts"), "const x = 1;\nawait handleAuth(request)\nreturn ok\n")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "payments-api", "notes.txt"), "handleAuth in docs\n")
	writeGrepWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
		},
	})

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"grep", "handleAuth", "--include", "*.go,*.ts,*.txt", "--exclude", "notes.txt", "--context", "1"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		"[auth-service]  src/middleware.go:2:  func handleAuth() {}",
		"[auth-service]  src/middleware.go:1-  package auth",
		"[auth-service]  src/middleware.go:3+  return handleAuth(ctx)",
		"[payments-api]  src/client.ts:2:  await handleAuth(request)",
		"[payments-api]  src/client.ts:1-  const x = 1;",
		"[payments-api]  src/client.ts:3+  return ok",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("grep output = %q, want substring %q", output, snippet)
		}
	}

	for _, unwanted := range []string{"ignored.txt", "notes.txt"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("grep output = %q, want %q excluded", output, unwanted)
		}
	}
}

func TestGrepJSONReturnsStructuredMatches(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "frontend", "src", "app.tsx"), "before\nconst token = refreshToken()\nafter\n")
	writeGrepWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"grep", "refreshToken", "--json", "--context", "1"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var matches []struct {
		Repo   string   `json:"repo"`
		File   string   `json:"file"`
		Line   int      `json:"line"`
		Match  string   `json:"match"`
		Before []string `json:"before"`
		After  []string `json:"after"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &matches); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	match := matches[0]
	if match.Repo != "frontend" {
		t.Fatalf("match.Repo = %q, want frontend", match.Repo)
	}
	if match.File != "src/app.tsx" {
		t.Fatalf("match.File = %q, want src/app.tsx", match.File)
	}
	if match.Line != 2 {
		t.Fatalf("match.Line = %d, want 2", match.Line)
	}
	if match.Match != "const token = refreshToken()" {
		t.Fatalf("match.Match = %q, want const token = refreshToken()", match.Match)
	}
	if len(match.Before) != 1 || match.Before[0] != "before" {
		t.Fatalf("match.Before = %#v, want [before]", match.Before)
	}
	if len(match.After) != 1 || match.After[0] != "after" {
		t.Fatalf("match.After = %#v, want [after]", match.After)
	}
}

func TestGrepReturnsNonZeroWhenNoMatchesFound(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	writeCmdGrepFile(t, filepath.Join(reposRoot, "frontend", "src", "app.tsx"), "export function App() {}\n")
	writeGrepWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stdout := new(bytes.Buffer)
	command := cmd.NewRootCommand()
	command.SetArgs([]string{"grep", "refreshToken"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want no-match failure")
	}
	if err.Error() != "no matches found" {
		t.Fatalf("ExecuteCommand() error = %q, want no matches found", err.Error())
	}
	if stdout.String() != "" {
		t.Fatalf("grep output = %q, want empty output", stdout.String())
	}
}

func writeGrepWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func writeCmdGrepFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
