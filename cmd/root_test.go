package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jwolf/wsx/cmd"
)

func TestRootHelpShowsSupportedCommands(t *testing.T) {
	command := cmd.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs([]string{"--help"})

	if err := cmd.ExecuteCommand(command); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		"Currently supported commands:",
		"add",
		"agent-init",
		"exec",
		"fetch",
		"grep",
		"init",
		"list",
		"remove",
		"status",
		"tree",
		"Only implemented commands are shown below.",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("help output = %q, want substring %q", output, snippet)
		}
	}
}

func TestUsageErrorPrintsHelp(t *testing.T) {
	command := cmd.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs([]string{"init", "one", "two"})

	err := cmd.ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want usage error")
	}

	output := stdout.String()
	for _, snippet := range []string{
		"Usage:",
		"init [name]",
		"help for init",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("help output = %q, want substring %q", output, snippet)
		}
	}
}
