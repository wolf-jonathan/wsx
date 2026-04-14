package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallBundledSkillLocalCopiesBundledSkill(t *testing.T) {
	repoRoot := t.TempDir()
	restoreReader := swapBundledSkillReader(func(string) ([]byte, error) {
		return []byte("# local skill\n"), nil
	})
	defer restoreReader()

	result, err := InstallBundledSkill(repoRoot, SkillScopeLocal)
	if err != nil {
		t.Fatalf("InstallBundledSkill() error = %v", err)
	}

	wantDir := filepath.Join(repoRoot, ".agents", "skills", SkillName)
	if result.Directory != wantDir {
		t.Fatalf("result.Directory = %q, want %q", result.Directory, wantDir)
	}

	data, err := os.ReadFile(result.SkillFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", result.SkillFile, err)
	}
	if string(data) != "# local skill\n" {
		t.Fatalf("installed skill = %q, want bundled content", string(data))
	}
}

func TestInstallBundledSkillGlobalUsesHomeSkillDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	restoreHome := swapSkillHomeDir(func() (string, error) {
		return homeDir, nil
	})
	defer restoreHome()
	restoreReader := swapBundledSkillReader(func(string) ([]byte, error) {
		return []byte("# global skill\n"), nil
	})
	defer restoreReader()

	result, err := InstallBundledSkill(repoRoot, SkillScopeGlobal)
	if err != nil {
		t.Fatalf("InstallBundledSkill() error = %v", err)
	}

	wantDir := filepath.Join(homeDir, ".agents", "skills", SkillName)
	if result.Directory != wantDir {
		t.Fatalf("result.Directory = %q, want %q", result.Directory, wantDir)
	}

	wantClaudeDir := filepath.Join(homeDir, ".claude", "skills", SkillName)
	if result.ClaudeDirectory != wantClaudeDir {
		t.Fatalf("result.ClaudeDirectory = %q, want %q", result.ClaudeDirectory, wantClaudeDir)
	}

	data, err := os.ReadFile(filepath.Join(result.ClaudeDirectory, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(Claude SKILL.md) error = %v", err)
	}
	if string(data) != "# global skill\n" {
		t.Fatalf("Claude-installed skill = %q, want bundled content", string(data))
	}

	if result.ClaudeLinkType == "" {
		t.Fatal("result.ClaudeLinkType = empty, want symlink or junction")
	}
}

func TestInstallBundledSkillRejectsExistingInstall(t *testing.T) {
	repoRoot := t.TempDir()
	installDir := filepath.Join(repoRoot, ".agents", "skills", SkillName)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if _, err := InstallBundledSkill(repoRoot, SkillScopeLocal); err == nil {
		t.Fatal("InstallBundledSkill() error = nil, want duplicate install error")
	}
}

func TestUninstallBundledSkillRemovesInstalledDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	installDir := filepath.Join(repoRoot, ".agents", "skills", SkillName)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := UninstallBundledSkill(repoRoot, SkillScopeLocal)
	if err != nil {
		t.Fatalf("UninstallBundledSkill() error = %v", err)
	}

	if _, err := os.Stat(result.Directory); !os.IsNotExist(err) {
		t.Fatalf("skill directory still exists after uninstall: stat err = %v", err)
	}
}

func TestUninstallBundledSkillGlobalRemovesClaudeMirror(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	restoreHome := swapSkillHomeDir(func() (string, error) {
		return homeDir, nil
	})
	defer restoreHome()
	restoreReader := swapBundledSkillReader(func(string) ([]byte, error) {
		return []byte("# global skill\n"), nil
	})
	defer restoreReader()

	if _, err := InstallBundledSkill(repoRoot, SkillScopeGlobal); err != nil {
		t.Fatalf("InstallBundledSkill() error = %v", err)
	}

	result, err := UninstallBundledSkill(repoRoot, SkillScopeGlobal)
	if err != nil {
		t.Fatalf("UninstallBundledSkill() error = %v", err)
	}

	if _, err := os.Lstat(result.ClaudeDirectory); !os.IsNotExist(err) {
		t.Fatalf("Claude skill path still exists after uninstall: lstat err = %v", err)
	}
	if _, err := os.Stat(result.Directory); !os.IsNotExist(err) {
		t.Fatalf("skill directory still exists after uninstall: stat err = %v", err)
	}
}

func TestUninstallBundledSkillFailsWhenMissing(t *testing.T) {
	repoRoot := t.TempDir()

	if _, err := UninstallBundledSkill(repoRoot, SkillScopeLocal); err == nil {
		t.Fatal("UninstallBundledSkill() error = nil, want missing install error")
	}
}

func swapSkillHomeDir(fn func() (string, error)) func() {
	previous := skillHomeDir
	skillHomeDir = fn
	return func() {
		skillHomeDir = previous
	}
}

func swapBundledSkillReader(fn func(string) ([]byte, error)) func() {
	previous := readBundledSkill
	readBundledSkill = fn
	return func() {
		readBundledSkill = previous
	}
}
