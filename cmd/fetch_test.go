package cmd

import (
	"bytes"
	"encoding/json"
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

func TestFetchShowsPerRepoSummariesAndReturnsErrorForFailures(t *testing.T) {
	root := t.TempDir()
	chdirForFetchTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api", "frontend"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeFetchWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stub := &stubFetchClient{
		results: map[string]stubFetchResult{
			filepath.Join(reposRoot, "auth-service"): {
				result: wsxgit.CommandResult{Stderr: "From origin\n * [new branch] main -> origin/main\n"},
			},
			filepath.Join(reposRoot, "payments-api"): {
				result: wsxgit.CommandResult{},
			},
			filepath.Join(reposRoot, "frontend"): {
				result: wsxgit.CommandResult{
					Stderr:   "fatal: unable to access remote\n",
					ExitCode: 128,
				},
				err: fmt.Errorf("git fetch failed"),
			},
		},
	}

	restore := swapFetchGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"fetch"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want fetch failure")
	}

	output := stdout.String()
	for _, snippet := range []string{
		"[auth-service]",
		"From origin | * [new branch] main -> origin/main",
		"[payments-api]",
		"fetched",
		"[frontend]",
		"error: fatal: unable to access remote",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("fetch output = %q, want substring %q", output, snippet)
		}
	}

	wantCalls := []string{
		filepath.Join(reposRoot, "auth-service"),
		filepath.Join(reposRoot, "payments-api"),
		filepath.Join(reposRoot, "frontend"),
	}
	assertFetchCallsEqual(t, stub.calls, wantCalls)
}

func TestFetchJSONIncludesResolvedPathsAndExitCodes(t *testing.T) {
	root := t.TempDir()
	chdirForFetchTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	writeFetchWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	stub := &stubFetchClient{
		results: map[string]stubFetchResult{
			target: {
				result: wsxgit.CommandResult{
					Stdout:   "Already up to date.\n",
					ExitCode: 0,
				},
			},
		},
	}

	restore := swapFetchGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"fetch", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var items []fetchItem
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
	if item.Summary != "Already up to date." {
		t.Fatalf("item.Summary = %q, want Already up to date.", item.Summary)
	}
	if item.ExitCode != 0 {
		t.Fatalf("item.ExitCode = %d, want 0", item.ExitCode)
	}
	if item.Error != "" {
		t.Fatalf("item.Error = %q, want empty", item.Error)
	}
}

func TestFetchParallelPreservesWorkspaceOrder(t *testing.T) {
	root := t.TempDir()
	chdirForFetchTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeFetchWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
		},
	})

	stub := newBlockingFetchClient()
	stub.results[filepath.Join(reposRoot, "auth-service")] = stubFetchResult{
		result: wsxgit.CommandResult{Stdout: "fetched auth\n"},
	}
	stub.results[filepath.Join(reposRoot, "payments-api")] = stubFetchResult{
		result: wsxgit.CommandResult{Stdout: "fetched payments\n"},
	}

	restore := swapFetchGitClient(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"fetch", "--parallel"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	done := make(chan error, 1)
	go func() {
		done <- ExecuteCommand(command)
	}()

	stub.waitForCalls(t, 2)
	stub.releaseAll()

	if err := <-done; err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	first := strings.Index(output, "[auth-service]")
	second := strings.Index(output, "[payments-api]")
	if first == -1 || second == -1 {
		t.Fatalf("fetch output = %q, want both repo headers", output)
	}
	if first > second {
		t.Fatalf("fetch output = %q, want workspace order preserved", output)
	}
}

type stubFetchClient struct {
	results map[string]stubFetchResult
	calls   []string
	mu      sync.Mutex
	blockCh chan struct{}
}

type stubFetchResult struct {
	result wsxgit.CommandResult
	err    error
}

func newBlockingFetchClient() *stubFetchClient {
	return &stubFetchClient{
		results: make(map[string]stubFetchResult),
		blockCh: make(chan struct{}),
	}
}

func (s *stubFetchClient) Fetch(path string) (wsxgit.CommandResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, path)
	blockCh := s.blockCh
	s.mu.Unlock()

	if blockCh != nil {
		<-blockCh
	}

	result, ok := s.results[path]
	if !ok {
		return wsxgit.CommandResult{}, fmt.Errorf("unexpected fetch path: %s", path)
	}

	return result.result, result.err
}

func (s *stubFetchClient) waitForCalls(t *testing.T, want int) {
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

func (s *stubFetchClient) releaseAll() {
	s.mu.Lock()
	blockCh := s.blockCh
	s.blockCh = nil
	s.mu.Unlock()

	if blockCh != nil {
		close(blockCh)
	}
}

func swapFetchGitClient(client gitFetchClient) func() {
	previous := newFetchGitClient
	newFetchGitClient = func() gitFetchClient {
		return client
	}

	return func() {
		newFetchGitClient = previous
	}
}

func writeFetchWorkspace(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func chdirForFetchTest(t *testing.T, dir string) {
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

func assertFetchCallsEqual(t *testing.T, got, want []string) {
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
