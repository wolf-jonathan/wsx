package git_test

import (
	"errors"
	"testing"

	wsxgit "github.com/wolf-jonathan/workspace-x/internal/git"
)

func TestStatusUsesGitShortBranchCommand(t *testing.T) {
	runner := &stubRunner{
		result: wsxgit.CommandResult{
			Stdout:   "## main\n",
			ExitCode: 0,
		},
	}

	client := wsxgit.NewClient(runner)
	result, err := client.Status(`C:\repos\auth-service`)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if runner.callCount != 1 {
		t.Fatalf("runner.callCount = %d, want 1", runner.callCount)
	}
	if runner.dir != `C:\repos\auth-service` {
		t.Fatalf("runner.dir = %q, want %q", runner.dir, `C:\repos\auth-service`)
	}
	if runner.name != "git" {
		t.Fatalf("runner.name = %q, want git", runner.name)
	}
	wantArgs := []string{"status", "--short", "--branch"}
	assertArgsEqual(t, runner.args, wantArgs)

	if result.Stdout != "## main\n" {
		t.Fatalf("result.Stdout = %q, want %q", result.Stdout, "## main\n")
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestFetchUsesGitFetchPruneCommand(t *testing.T) {
	runner := &stubRunner{
		result: wsxgit.CommandResult{
			Stdout:   "From origin\n",
			ExitCode: 0,
		},
	}

	client := wsxgit.NewClient(runner)
	result, err := client.Fetch(`C:\repos\payments-api`)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if runner.callCount != 1 {
		t.Fatalf("runner.callCount = %d, want 1", runner.callCount)
	}
	if runner.dir != `C:\repos\payments-api` {
		t.Fatalf("runner.dir = %q, want %q", runner.dir, `C:\repos\payments-api`)
	}
	if runner.name != "git" {
		t.Fatalf("runner.name = %q, want git", runner.name)
	}
	wantArgs := []string{"fetch", "--prune"}
	assertArgsEqual(t, runner.args, wantArgs)

	if result.Stdout != "From origin\n" {
		t.Fatalf("result.Stdout = %q, want %q", result.Stdout, "From origin\n")
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestStatusPropagatesRunnerErrorsAndExitData(t *testing.T) {
	runErr := errors.New("git exited with status 1")
	runner := &stubRunner{
		result: wsxgit.CommandResult{
			Stderr:   "fatal: not a git repository\n",
			ExitCode: 1,
		},
		err: runErr,
	}

	client := wsxgit.NewClient(runner)
	result, err := client.Status(`C:\repos\broken`)
	if !errors.Is(err, runErr) {
		t.Fatalf("Status() error = %v, want %v", err, runErr)
	}
	if result.ExitCode != 1 {
		t.Fatalf("result.ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Stderr != "fatal: not a git repository\n" {
		t.Fatalf("result.Stderr = %q, want %q", result.Stderr, "fatal: not a git repository\n")
	}
}

type stubRunner struct {
	dir       string
	name      string
	args      []string
	result    wsxgit.CommandResult
	err       error
	callCount int
}

func (s *stubRunner) Run(dir, name string, args ...string) (wsxgit.CommandResult, error) {
	s.callCount++
	s.dir = dir
	s.name = name
	s.args = append([]string(nil), args...)
	return s.result, s.err
}

func assertArgsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(args) = %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full args: %v)", i, got[i], want[i], got)
		}
	}
}
