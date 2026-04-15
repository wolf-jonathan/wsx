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

func TestExecShowsGroupedOutputAndReturnsErrorForFailures(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api", "frontend"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeExecWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
			{Name: "frontend", Path: filepath.Join(reposRoot, "frontend")},
		},
	})

	stub := &stubExecRunner{
		results: map[string]stubExecResult{
			filepath.Join(reposRoot, "auth-service"): {
				result: wsxgit.CommandResult{Stdout: "Already up to date.\n"},
			},
			filepath.Join(reposRoot, "payments-api"): {
				result: wsxgit.CommandResult{},
			},
			filepath.Join(reposRoot, "frontend"): {
				result: wsxgit.CommandResult{
					Stderr:   "fatal: bad revision 'main'\n",
					ExitCode: 128,
				},
				err: fmt.Errorf("git [checkout main]: exit status 128"),
			},
		},
	}

	restore := swapExecRunner(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"exec", "--", "git", "checkout", "main"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want exec failure")
	}

	output := stdout.String()
	for _, snippet := range []string{
		"[auth-service]",
		"Already up to date.",
		"[payments-api]",
		"(no output)",
		"[frontend]",
		"fatal: bad revision 'main'",
		"command failed with exit code 128",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("exec output = %q, want substring %q", output, snippet)
		}
	}

	wantCalls := []stubExecCall{
		{Dir: filepath.Join(reposRoot, "auth-service"), Name: "git", Args: []string{"checkout", "main"}},
		{Dir: filepath.Join(reposRoot, "payments-api"), Name: "git", Args: []string{"checkout", "main"}},
		{Dir: filepath.Join(reposRoot, "frontend"), Name: "git", Args: []string{"checkout", "main"}},
	}
	assertExecCallsEqual(t, stub.calls, wantCalls)
}

func TestExecJSONIncludesCommandOutputAndResolvedPaths(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	writeExecWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	stub := &stubExecRunner{
		results: map[string]stubExecResult{
			target: {
				result: wsxgit.CommandResult{
					Stdout:   "lint ok\n",
					Stderr:   "warning: cached result\n",
					ExitCode: 0,
				},
			},
		},
	}

	restore := swapExecRunner(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"exec", "--json", "--", "npm", "run", "lint"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var items []execItem
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
	if strings.Join(item.Command, " ") != "npm run lint" {
		t.Fatalf("item.Command = %v, want [npm run lint]", item.Command)
	}
	if item.Stdout != "lint ok\n" {
		t.Fatalf("item.Stdout = %q, want lint ok", item.Stdout)
	}
	if item.Stderr != "warning: cached result\n" {
		t.Fatalf("item.Stderr = %q, want warning", item.Stderr)
	}
	if item.ExitCode != 0 {
		t.Fatalf("item.ExitCode = %d, want 0", item.ExitCode)
	}
	if item.Error != "" {
		t.Fatalf("item.Error = %q, want empty", item.Error)
	}
}

func TestExecTreatsNonZeroExitCodeAsFailureWithoutRunnerError(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	writeExecWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	stub := &stubExecRunner{
		results: map[string]stubExecResult{
			target: {
				result: wsxgit.CommandResult{
					Stdout:   "tests failed\n",
					ExitCode: 3,
				},
			},
		},
	}

	restore := swapExecRunner(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"exec", "--json", "--", "go", "test", "./..."})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want exec failure")
	}

	var items []execItem
	if decodeErr := json.Unmarshal(stdout.Bytes(), &items); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	if items[0].ExitCode != 3 {
		t.Fatalf("item.ExitCode = %d, want 3", items[0].ExitCode)
	}
	if items[0].Error != "" {
		t.Fatalf("item.Error = %q, want empty", items[0].Error)
	}
}

func TestExecParallelPreservesWorkspaceOrder(t *testing.T) {
	root := t.TempDir()
	chdirForExecTest(t, root)

	reposRoot := filepath.Join(t.TempDir(), "repos")
	for _, name := range []string{"auth-service", "payments-api"} {
		if err := os.MkdirAll(filepath.Join(reposRoot, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
	}
	writeExecWorkspace(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: filepath.Join(reposRoot, "auth-service")},
			{Name: "payments-api", Path: filepath.Join(reposRoot, "payments-api")},
		},
	})

	stub := newBlockingExecRunner()
	stub.results[filepath.Join(reposRoot, "auth-service")] = stubExecResult{
		result: wsxgit.CommandResult{Stdout: "lint auth\n"},
	}
	stub.results[filepath.Join(reposRoot, "payments-api")] = stubExecResult{
		result: wsxgit.CommandResult{Stdout: "lint payments\n"},
	}

	restore := swapExecRunner(stub)
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"exec", "--parallel", "--", "npm", "run", "lint"})
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
		t.Fatalf("exec output = %q, want both repo headers", output)
	}
	if first > second {
		t.Fatalf("exec output = %q, want workspace order preserved", output)
	}
}

type stubExecRunner struct {
	results map[string]stubExecResult
	calls   []stubExecCall
	mu      sync.Mutex
	blockCh chan struct{}
}

type stubExecResult struct {
	result wsxgit.CommandResult
	err    error
}

type stubExecCall struct {
	Dir  string
	Name string
	Args []string
}

func newBlockingExecRunner() *stubExecRunner {
	return &stubExecRunner{
		results: make(map[string]stubExecResult),
		blockCh: make(chan struct{}),
	}
}

func (s *stubExecRunner) Run(dir, name string, args ...string) (wsxgit.CommandResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, stubExecCall{
		Dir:  dir,
		Name: name,
		Args: append([]string(nil), args...),
	})
	blockCh := s.blockCh
	s.mu.Unlock()

	if blockCh != nil {
		<-blockCh
	}

	result, ok := s.results[dir]
	if !ok {
		return wsxgit.CommandResult{}, fmt.Errorf("unexpected exec dir: %s", dir)
	}

	return result.result, result.err
}

func (s *stubExecRunner) waitForCalls(t *testing.T, want int) {
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

func (s *stubExecRunner) releaseAll() {
	s.mu.Lock()
	blockCh := s.blockCh
	s.blockCh = nil
	s.mu.Unlock()

	if blockCh != nil {
		close(blockCh)
	}
}

func swapExecRunner(runner commandRunner) func() {
	previous := newExecRunner
	newExecRunner = func() commandRunner {
		return runner
	}

	return func() {
		newExecRunner = previous
	}
}

func writeExecWorkspace(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func chdirForExecTest(t *testing.T, dir string) {
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

func assertExecCallsEqual(t *testing.T, got, want []stubExecCall) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(calls) = %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i].Dir != want[i].Dir || got[i].Name != want[i].Name || strings.Join(got[i].Args, "\x00") != strings.Join(want[i].Args, "\x00") {
			t.Fatalf("calls[%d] = %+v, want %+v (full calls: %+v)", i, got[i], want[i], got)
		}
	}
}
