package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestDoctorReportsHealthyWorkspace(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	writeDoctorEnvFile(t, root, "WORK_REPOS="+reposRoot+"\n")
	if _, err := workspace.CreateLink(target, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	restore := swapDoctorTerminalDetector(func() bool { return false })
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String()
	for _, snippet := range []string{
		"OK  config_valid",
		"OK  env_file",
		"OK  var_WORK_REPOS",
		"OK  auth-service_link",
		"OK  no_duplicate_names",
		"OK  no_case_collisions",
		"OK  no_workspace_nesting",
		"OK  no_nested_refs",
		"OK  auth-service_git",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("doctor output = %q, want substring %q", output, snippet)
		}
	}
}

func TestDoctorJSONReportsUnresolvedVariableInNonInteractiveMode(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	restore := swapDoctorTerminalDetector(func() bool { return false })
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want unresolved variable failure")
	}

	var result doctorReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}

	foundEnvWarning := false
	foundVarError := false
	for _, check := range result.Checks {
		if check.Name == "env_file" && check.Status == doctorStatusWarn {
			foundEnvWarning = true
		}
		if check.Name == "var_WORK_REPOS" && check.Status == doctorStatusError {
			foundVarError = true
		}
	}

	if !foundEnvWarning {
		t.Fatalf("result.Checks = %+v, want env_file warning", result.Checks)
	}
	if !foundVarError {
		t.Fatalf("result.Checks = %+v, want var_WORK_REPOS error", result.Checks)
	}
}

func TestDoctorFixRequiresInteractiveTerminal(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
	})

	restore := swapDoctorTerminalDetector(func() bool { return false })
	defer restore()

	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--fix"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want --fix terminal error")
	}
	if !strings.Contains(err.Error(), "--fix requires an interactive terminal") {
		t.Fatalf("ExecuteCommand() error = %q, want --fix terminal error", err.Error())
	}
}

func TestDoctorDoesNotPromptOrWriteEnvWithoutFix(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	restore := swapDoctorTerminalDetector(func() bool { return true })
	defer restore()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor"})
	command.SetIn(strings.NewReader("C:\\repos\n"))
	command.SetOut(stdout)
	command.SetErr(stderr)

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want unresolved variable failure")
	}

	if strings.Contains(stdout.String(), "Enter the path for WORK_REPOS") {
		t.Fatalf("doctor stdout = %q, want no prompt without --fix", stdout.String())
	}
	if strings.Contains(stderr.String(), "Enter the path for WORK_REPOS") {
		t.Fatalf("doctor stderr = %q, want no prompt without --fix", stderr.String())
	}
	if _, statErr := os.Stat(filepath.Join(root, workspace.EnvFileName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Stat(.wsx.env) error = %v, want not exists", statErr)
	}
}

func TestDoctorInteractiveModeResolvesVariablesAndWritesEnvFile(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	if _, err := workspace.CreateLink(target, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	restore := swapDoctorTerminalDetector(func() bool { return true })
	defer restore()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--fix"})
	command.SetIn(strings.NewReader(reposRoot + "\n"))
	command.SetOut(stdout)
	command.SetErr(stderr)

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	output := stdout.String() + stderr.String()
	for _, snippet := range []string{
		"Enter the path for WORK_REPOS",
		"Saved WORK_REPOS to .wsx.env",
		"OK  var_WORK_REPOS",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("doctor output = %q, want substring %q", output, snippet)
		}
	}

	content, err := os.ReadFile(filepath.Join(root, workspace.EnvFileName))
	if err != nil {
		t.Fatalf("ReadFile(.wsx.env) error = %v", err)
	}
	if string(content) != "WORK_REPOS="+reposRoot+"\n" {
		t.Fatalf(".wsx.env = %q, want WORK_REPOS entry", string(content))
	}
}

func TestDoctorFixJSONKeepsStdoutMachineReadable(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	target := filepath.Join(reposRoot, "auth-service")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	if _, err := workspace.CreateLink(target, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	restore := swapDoctorTerminalDetector(func() bool { return true })
	defer restore()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--fix", "--json"})
	command.SetIn(strings.NewReader(reposRoot + "\n"))
	command.SetOut(stdout)
	command.SetErr(stderr)

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var result doctorReport
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout = %q", err, stdout.String())
	}

	if !strings.Contains(stderr.String(), "Enter the path for WORK_REPOS") {
		t.Fatalf("doctor stderr = %q, want prompt output", stderr.String())
	}
	if strings.Contains(stdout.String(), "Enter the path for WORK_REPOS") {
		t.Fatalf("doctor stdout = %q, want JSON only", stdout.String())
	}
}

func TestDoctorReportsRepointedWorkspaceLinkAsBroken(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)
	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	reposRoot := filepath.Join(t.TempDir(), "repos")
	configuredTarget := filepath.Join(reposRoot, "auth-service")
	actualTarget := filepath.Join(reposRoot, "other-service")
	for _, dir := range []string{configuredTarget, actualTarget} {
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("MkdirAll(.git) error = %v", err)
		}
	}
	writeDoctorEnvFile(t, root, "WORK_REPOS="+reposRoot+"\n")
	if _, err := workspace.CreateLink(actualTarget, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	restore := swapDoctorTerminalDetector(func() bool { return false })
	defer restore()

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want unhealthy workspace failure")
	}

	var result doctorReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	for _, check := range result.Checks {
		if check.Name != "auth-service_link" {
			continue
		}
		if check.Status != doctorStatusError {
			t.Fatalf("auth-service_link status = %q, want %q", check.Status, doctorStatusError)
		}
		if !strings.Contains(check.Message, "instead of") {
			t.Fatalf("auth-service_link message = %q, want repointed-link detail", check.Message)
		}
		return
	}

	t.Fatalf("result.Checks = %+v, want auth-service_link error", result.Checks)
}

func writeDoctorWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
}

func writeDoctorEnvFile(t *testing.T, root, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, workspace.EnvFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(.wsx.env) error = %v", err)
	}
}

func chdirForDoctorTest(t *testing.T, dir string) {
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

func swapDoctorTerminalDetector(detector func() bool) func() {
	previous := doctorIsTerminal
	doctorIsTerminal = detector

	return func() {
		doctorIsTerminal = previous
	}
}
