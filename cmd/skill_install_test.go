package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillInstallLocalCreatesRepoScopedSkill(t *testing.T) {
	root := t.TempDir()
	chdirForSkillCommandTest(t, root)
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# bundled skill\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"skill-install"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	installedPath := filepath.Join(root, ".agents", "skills", "wsx", "SKILL.md")
	data, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", installedPath, err)
	}
	if string(data) != "# bundled skill\n" {
		t.Fatalf("installed skill = %q, want bundled skill", string(data))
	}
	if !strings.Contains(stdout.String(), filepath.Dir(installedPath)) {
		t.Fatalf("stdout = %q, want install directory", stdout.String())
	}
}

func TestSkillInstallGlobalUsesHomeScope(t *testing.T) {
	root := t.TempDir()
	homeDir := t.TempDir()
	chdirForSkillCommandTest(t, root)

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	stdout := new(bytes.Buffer)
	command := NewRootCommand()
	command.SetArgs([]string{"skill-install", "--scope", "global"})
	command.SetOut(stdout)
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	installedPath := filepath.Join(homeDir, ".agents", "skills", "wsx", "SKILL.md")
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("Stat(%q) error = %v", installedPath, err)
	}

	claudeInstalledPath := filepath.Join(homeDir, ".claude", "skills", "wsx", "SKILL.md")
	if _, err := os.Stat(claudeInstalledPath); err != nil {
		t.Fatalf("Stat(%q) error = %v", claudeInstalledPath, err)
	}
	if !strings.Contains(stdout.String(), filepath.Join(homeDir, ".claude", "skills", "wsx")) {
		t.Fatalf("stdout = %q, want Claude skill directory", stdout.String())
	}
}

func TestSkillInstallOverridesExistingInstall(t *testing.T) {
	root := t.TempDir()
	chdirForSkillCommandTest(t, root)
	installDir := filepath.Join(root, ".agents", "skills", "wsx")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "SKILL.md"), []byte("# old install\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# bundled replacement\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	command := NewRootCommand()
	command.SetArgs([]string{"skill-install"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(installDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}
	if string(data) != "# bundled replacement\n" {
		t.Fatalf("installed skill = %q, want replacement content", string(data))
	}
}

func TestSkillUninstallRemovesInstalledSkill(t *testing.T) {
	root := t.TempDir()
	chdirForSkillCommandTest(t, root)
	installDir := filepath.Join(root, ".agents", "skills", "wsx")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "SKILL.md"), []byte("# installed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	command := NewRootCommand()
	command.SetArgs([]string{"skill-uninstall"})
	command.SetOut(new(bytes.Buffer))
	command.SetErr(new(bytes.Buffer))

	if err := ExecuteCommand(command); err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}

	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("skill directory still exists after uninstall: stat err = %v", err)
	}
}

func chdirForSkillCommandTest(t *testing.T, dir string) {
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
