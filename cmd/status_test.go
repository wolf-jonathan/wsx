package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	wsxgit "github.com/wolf-jonathan/workspace-x/internal/git"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestStatusShowsPerRepoSummariesAndReturnsErrorForDirtyRepos(t *testing.T) {
	root := t.TempDir()
	chdirForStatusTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api", "frontend"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeStatusWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stub := &stubStatusClient{
		results: map[string]stubStatusResult{
			filepath.Join(reposRoot, "auth-service"): {
				result: wsxgit.CommandResult{Stdout: "## main...origin/main [ahead 1]\n M app.go\n?? notes.txt\n"},
			},
			filepath.Join(reposRoot, "payments-api"): {
				result: wsxgit.CommandResult{Stdout: "## main...origin/main\n"},
			},
			filepath.Join(reposRoot, "frontend"): {
				result: wsxgit.CommandResult{
					Stderr:   "fatal: not a git repository\n",
					ExitCode: 128,
				},
				err: errors.New("git status failed"),
			},
		},
	}

	restore := swapStatusGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"status"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want dirty status error")
	}

	output := stdout.String()
	for _, snippet := range []string{
		"[auth-service]",
		"main...origin/main [ahead 1]; 1 file changed, 1 untracked file",
		"[payments-api]",
		"main...origin/main; clean",
		"[frontend]",
		"error: fatal: not a git repository",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("status output = %q, want substring %q", output, snippet)
		}
	}

	wantCalls := []string{
		filepath.Join(reposRoot, "auth-service"),
		filepath.Join(reposRoot, "payments-api"),
		filepath.Join(reposRoot, "frontend"),
	}
	assertStatusCallsEqual(t, stub.calls, wantCalls)
}

func TestStatusJSONIncludesResolvedPathsAndExitCodes(t *testing.T) {
	root := t.TempDir()
	chdirForStatusTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	writeStatusWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	stub := &stubStatusClient{
		results: map[string]stubStatusResult{
			target: {
				result: wsxgit.CommandResult{
					Stdout:   "## feature/payments...origin/feature/payments [ahead 2, behind 1]\n M app.go\n",
					ExitCode: 0,
				},
			},
		},
	}

	restore := swapStatusGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"status", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want dirty status error")
	}

	var items []statusItem
	if decodeErr := json.Unmarshal(stdout.Bytes(), &items); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	item := items[0]
	if item.Name != "auth-service" {
		t.Fatalf("item.Name = %q, want auth-service", item.Name)
	}
	if item.Path != target {
		t.Fatalf("item.Path = %q, want %q", item.Path, target)
	}
	if item.ResolvedPath != target {
		t.Fatalf("item.ResolvedPath = %q, want %q", item.ResolvedPath, target)
	}
	if item.Summary != "feature/payments...origin/feature/payments [ahead 2, behind 1]; 1 file changed" {
		t.Fatalf("item.Summary = %q, want branch summary plus changes", item.Summary)
	}
	if item.Clean {
		t.Fatal("item.Clean = true, want false")
	}
	if item.ExitCode != 0 {
		t.Fatalf("item.ExitCode = %d, want 0", item.ExitCode)
	}
	if item.Error != "" {
		t.Fatalf("item.Error = %q, want empty", item.Error)
	}
}

func TestStatusSucceedsWhenAllReposAreClean(t *testing.T) {
	root := t.TempDir()
	chdirForStatusTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "payments-api")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	writeStatusWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "payments-api", Path: target},
		},
	})

	stub := &stubStatusClient{
		results: map[string]stubStatusResult{
			target: {
				result: wsxgit.CommandResult{Stdout: "## HEAD (detached at a1b2c3d)\n"},
			},
		},
	}

	restore := swapStatusGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"status"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "HEAD (detached at a1b2c3d); clean") {
		t.Fatalf("status output = %q, want detached clean summary", stdout.String())
	}
}

func TestStatusParallelPreservesWorkspaceOrder(t *testing.T) {
	root := t.TempDir()
	chdirForStatusTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeStatusWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
		},
	})

	stub := newBlockingStatusClient()
	stub.results[filepath.Join(reposRoot, "auth-service")] = stubStatusResult{
		result: wsxgit.CommandResult{Stdout: "## main...origin/main\n"},
	}
	stub.results[filepath.Join(reposRoot, "payments-api")] = stubStatusResult{
		result: wsxgit.CommandResult{Stdout: "## release...origin/release [behind 1]\n M api.go\n"},
	}

	restore := swapStatusGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"status", "--parallel"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	done := make(chan error, 1)
	go func() {
		done <- ExecuteCommand(command)
	}()

	stub.waitForCalls(t, 2)
	stub.releaseAll()

	if err := <-done; err == nil {
		t.Fatal("ExecuteCommand() error = nil, want dirty status error")
	}

	output := stdout.String()
	first := strings.Index(output, "[auth-service]")
	second := strings.Index(output, "[payments-api]")
	if first == -1 || second == -1 {
		t.Fatalf("status output = %q, want both repo headers", output)
	}
	if first > second {
		t.Fatalf("status output = %q, want workspace order preserved", output)
	}
}

type stubStatusClient struct {
	results map[string]stubStatusResult
	calls   []string
	mu      sync.Mutex
	blockCh chan struct{}
}

type stubStatusResult struct {
	result wsxgit.CommandResult
	err    error
}

func (s *stubStatusClient) Status(path string) (wsxgit.CommandResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, path)
	blockCh := s.blockCh
	s.mu.Unlock()

	if blockCh != nil {
		<-blockCh
	}

	result, ok := s.results[path]
	if !ok {
		return wsxgit.CommandResult{}, fmt.Errorf("unexpected status path: %s", path)
	}

	return result.result, result.err
}

func newBlockingStatusClient() *stubStatusClient {
	return &stubStatusClient{
		results: make(map[string]stubStatusResult),
		blockCh: make(chan struct{}),
	}
}

func (s *stubStatusClient) waitForCalls(t *testing.T, want int) {
	t.Helper()

	for i := 0; i < 100; i++ {
		s.mu.Lock()
		got := len(s.calls)
		s.mu.Unlock()
		if got >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	t.Fatalf("len(calls) = %d, want at least %d", len(s.calls), want)
}

func (s *stubStatusClient) releaseAll() {
	s.mu.Lock()
	blockCh := s.blockCh
	s.blockCh = nil
	s.mu.Unlock()

	if blockCh != nil {
		close(blockCh)
	}
}

func swapStatusGitClient(client gitStatusClient) func() {
	previous := newStatusGitClient
	newStatusGitClient = func() gitStatusClient {
		return client
	}

	return func() {
		newStatusGitClient = previous
	}
}

func writeStatusWorkspace(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func chdirForStatusTest(t *testing.T, dir string) {
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

func assertStatusCallsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(calls) = %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q (full calls: %v)", i, got[i], want[i], got)
		}
	}
}
