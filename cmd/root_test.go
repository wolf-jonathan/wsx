package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wolf-jonathan/workspace-x/cmd"
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
		"Available Commands:",
		"add",
		"agent-init",
		"doctor",
		"exec",
		"fetch",
		"grep",
		"init",
		"list",
		"favorite",
		"prompt",
		"remove",
		"skill-install",
		"skill-uninstall",
		"status",
		"tree",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("help output = %q, want substring %q", output, snippet)
		}
	}
}

func TestNewRootCommandDisablesWindowsExplorerDelay(t *testing.T) {
	previous := cobra.MousetrapHelpText
	t.Cleanup(func() {
		cobra.MousetrapHelpText = previous
	})

	cobra.MousetrapHelpText = "enabled"

	cmd.NewRootCommand()

	if cobra.MousetrapHelpText != "" {
		t.Fatalf("cobra.MousetrapHelpText = %q, want empty string", cobra.MousetrapHelpText)
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
