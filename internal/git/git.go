package git

import (
	"bytes"
	"fmt"
	"os/exec"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(dir, name string, args ...string) (CommandResult, error)
}

type Client struct {
	runner Runner
}

type ExecRunner struct{}

func NewClient(runner Runner) *Client {
	if runner == nil {
		runner = ExecRunner{}
	}

	return &Client{runner: runner}
}

func Status(path string) (CommandResult, error) {
	return NewClient(nil).Status(path)
}

func Fetch(path string) (CommandResult, error) {
	return NewClient(nil).Fetch(path)
}

func (c *Client) Status(path string) (CommandResult, error) {
	return c.runner.Run(path, "git", "status", "--short", "--branch")
}

func (c *Client) Fetch(path string) (CommandResult, error) {
	return c.runner.Run(path, "git", "fetch", "--prune")
}

func (ExecRunner) Run(dir, name string, args ...string) (CommandResult, error) {
	command := exec.Command(name, args...)
	command.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}

	if err != nil {
		if result.ExitCode == 0 {
			result.ExitCode = 1
		}
		return result, fmt.Errorf("%s %v: %w", name, args, err)
	}

	return result, nil
}
