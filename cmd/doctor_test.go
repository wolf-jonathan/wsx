package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wolf-jonathan/workspace-x/internal/ai"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestDoctorReportsHealthyWorkspace(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)

	target := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}

	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	if _, err := workspace.CreateLink(target, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	writeDoctorInstructionFiles(t, root, buildDoctorInstructionContent(t, "payments-debug", []ai.InstructionRepo{{Name: "auth-service", Root: target}}))

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
		"OK  stored_paths_valid",
		"OK  auth-service_link",
		"OK  no_duplicate_names",
		"OK  no_case_collisions",
		"OK  no_workspace_nesting",
		"OK  no_nested_refs",
		"OK  auth-service_git",
		"OK  workspace_instruction_files",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("doctor output = %q, want substring %q", output, snippet)
		}
	}
}

func TestDoctorWarnsWhenWorkspaceInstructionFilesAreMissing(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)

	target := filepath.Join(t.TempDir(), "auth-service")
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}

	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: target},
		},
	})

	if _, err := workspace.CreateLink(target, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	var result doctorReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	if !result.Healthy {
		t.Fatal("result.Healthy = false, want true")
	}

	check, ok := findDoctorCheck(result.Checks, "workspace_instruction_files")
	if !ok {
		t.Fatalf("result.Checks = %+v, want workspace_instruction_files warning", result.Checks)
	}
	if check.Status != doctorStatusWarn {
		t.Fatalf("workspace_instruction_files status = %q, want %q", check.Status, doctorStatusWarn)
	}
	if !strings.Contains(check.Message, "missing") {
		t.Fatalf("workspace_instruction_files message = %q, want missing warning", check.Message)
	}
}

func TestDoctorReportsLegacyPlaceholderPathsAsErrors(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)

	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `${WORK_REPOS}/auth-service`},
		},
	})

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want invalid path failure")
	}

	var result doctorReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}

	check, ok := findDoctorCheck(result.Checks, "stored_paths_valid")
	if !ok {
		t.Fatalf("result.Checks = %+v, want stored_paths_valid error", result.Checks)
	}
	if check.Status != doctorStatusError {
		t.Fatalf("stored_paths_valid status = %q, want %q", check.Status, doctorStatusError)
	}
	if !strings.Contains(check.Message, "legacy placeholder-based workspace config") {
		t.Fatalf("stored_paths_valid message = %q, want legacy placeholder error", check.Message)
	}
}

func TestDoctorReportsRelativeStoredPathsAsErrors(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)

	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: `repos\auth-service`},
		},
	})

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"doctor", "--json"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	err := ExecuteCommand(command)
	if err == nil {
		t.Fatal("ExecuteCommand() error = nil, want invalid path failure")
	}

	var result doctorReport
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", decodeErr)
	}

	check, ok := findDoctorCheck(result.Checks, "stored_paths_valid")
	if !ok {
		t.Fatalf("result.Checks = %+v, want stored_paths_valid error", result.Checks)
	}
	if check.Status != doctorStatusError {
		t.Fatalf("stored_paths_valid status = %q, want %q", check.Status, doctorStatusError)
	}
	if !strings.Contains(check.Message, "must be absolute") {
		t.Fatalf("stored_paths_valid message = %q, want absolute-path error", check.Message)
	}
}

func TestDoctorReportsRepointedWorkspaceLinkAsBroken(t *testing.T) {
	root := t.TempDir()
	chdirForDoctorTest(t, root)

	configuredTarget := filepath.Join(t.TempDir(), "auth-service")
	actualTarget := filepath.Join(t.TempDir(), "other-service")
	for _, dir := range []string{configuredTarget, actualTarget} {
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("MkdirAll(.git) error = %v", err)
		}
	}

	writeDoctorWorkspaceConfig(t, root, workspace.Config{
		Version: "2",
		Name:    "payments-debug",
		Refs: []workspace.Ref{
			{Name: "auth-service", Path: configuredTarget},
		},
	})

	if _, err := workspace.CreateLink(actualTarget, filepath.Join(root, "auth-service")); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

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

func buildDoctorInstructionContent(t *testing.T, workspaceName string, repos []ai.InstructionRepo) string {
	t.Helper()

	content, err := ai.BuildWorkspaceInstructionContent(workspaceName, "", repos)
	if err != nil {
		t.Fatalf("BuildWorkspaceInstructionContent() error = %v", err)
	}

	return content
}

func writeDoctorInstructionFiles(t *testing.T, root, content string) {
	t.Helper()

	for _, relativePath := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(root, relativePath)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", relativePath, err)
		}
	}
}

func findDoctorCheck(checks []doctorCheck, name string) (doctorCheck, bool) {
	for _, check := range checks {
		if check.Name == name {
			return check, true
		}
	}

	return doctorCheck{}, false
}

func writeDoctorWorkspaceConfig(t *testing.T, root string, cfg workspace.Config) {
	t.Helper()

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
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
