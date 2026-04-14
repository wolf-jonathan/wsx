package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

const (
	SkillScopeLocal  = "local"
	SkillScopeGlobal = "global"
	SkillName        = "wsx"
)

const defaultBundledSkill = `---
name: wsx
description: Windows-first AI workspace manager for linked local repositories. Use when operating inside a wsx workspace or when a user needs structured inspection, health checks, multi-repo execution, or agent instruction setup across linked repos. Prefer tree for discovery, grep for narrowing, and exact-file reads instead of broad content extraction.
---

# Workspace X (wsx)

Use Workspace X through the wsx command when you need to inspect or operate
on a multi-repo workspace built from links to existing local repositories.

## What wsx manages

- A workspace root containing .wsx.json, .wsx.env, and linked repo directories.
- Portable committed config in .wsx.json.
- Local machine path variables in .wsx.env.
- Symlinks or Windows junctions created at the workspace root.

## Required invariants

- Keep ${VAR} placeholders in .wsx.json when available. Do not rewrite stored paths to machine-specific absolute paths.
- Resolve ${VAR} placeholders only at point of use.
- Treat .wsx.env as local-only state. It should be gitignored and never committed from generated workspaces.
- Treat link_type as runtime state. Detect it from disk instead of storing it in .wsx.json.
- On Windows, expect link creation to try symlinks first and fall back to junctions on permission errors.
- wsx exec forwards argv directly. Shell operators work only if the caller explicitly invokes a shell.

## Recommended workflow

1. Run wsx doctor --json first when you enter an unfamiliar workspace.
2. Use wsx list --json to inspect linked repos and resolved paths.
3. Use wsx tree for workspace discovery.
4. Use wsx grep to narrow to exact files or symbols before reading content.
5. Use wsx status --json, wsx fetch --json, or wsx exec --json -- ... for structured multi-repo automation.
6. Use wsx prompt only when the user explicitly needs a reusable workspace prompt.

## Command guidance

- wsx init: creates .wsx.json, .wsx.env, and ensures .wsx.env is in .gitignore.
- wsx add: accepts absolute or parameterized paths, rejects circular refs, and creates the runtime link.
- wsx remove: removes the workspace link and config entry only. It must not touch the target repo.
- wsx list: reports live link health and runtime link type.
- wsx doctor: distinguishes interactive TTY use from non-interactive agent or CI use.
- wsx tree: use this first for cheap workspace discovery.
- wsx grep: use this after tree to narrow scope before reading files.
`

type SkillInstallResult struct {
	Scope           string
	Directory       string
	SkillFile       string
	ClaudeDirectory string
	ClaudeLinkType  string
}

var skillHomeDir = os.UserHomeDir

var readBundledSkill = func(repoRoot string) ([]byte, error) {
	path := filepath.Join(repoRoot, "SKILL.md")
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return []byte(defaultBundledSkill), nil
	}
	return nil, err
}

func InstallBundledSkill(repoRoot, scope string) (SkillInstallResult, error) {
	location, err := resolveSkillInstallLocation(repoRoot, scope)
	if err != nil {
		return SkillInstallResult{}, err
	}

	if _, err := os.Stat(location.Directory); err == nil {
		return SkillInstallResult{}, fmt.Errorf("skill already installed at %s", location.Directory)
	} else if !errors.Is(err, os.ErrNotExist) {
		return SkillInstallResult{}, err
	}

	content, err := readBundledSkill(repoRoot)
	if err != nil {
		return SkillInstallResult{}, err
	}

	if err := os.MkdirAll(location.Directory, 0o755); err != nil {
		return SkillInstallResult{}, err
	}

	if err := os.WriteFile(location.SkillFile, content, 0o644); err != nil {
		return SkillInstallResult{}, err
	}

	if location.ClaudeDirectory != "" {
		linkType, err := createClaudeSkillLink(location.Directory, location.ClaudeDirectory)
		if err != nil {
			_ = os.RemoveAll(location.Directory)
			return SkillInstallResult{}, err
		}
		location.ClaudeLinkType = linkType
	}

	return location, nil
}

func UninstallBundledSkill(repoRoot, scope string) (SkillInstallResult, error) {
	location, err := resolveSkillInstallLocation(repoRoot, scope)
	if err != nil {
		return SkillInstallResult{}, err
	}

	if _, err := os.Stat(location.SkillFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SkillInstallResult{}, fmt.Errorf("skill is not installed at %s", location.Directory)
		}
		return SkillInstallResult{}, err
	}

	if location.ClaudeDirectory != "" {
		if err := removeClaudeSkillLink(location.ClaudeDirectory); err != nil {
			return SkillInstallResult{}, err
		}
	}

	if err := os.RemoveAll(location.Directory); err != nil {
		return SkillInstallResult{}, err
	}

	return location, nil
}

func resolveSkillInstallLocation(repoRoot, scope string) (SkillInstallResult, error) {
	normalizedScope := strings.ToLower(strings.TrimSpace(scope))
	switch normalizedScope {
	case SkillScopeLocal:
		root := filepath.Clean(repoRoot)
		directory := filepath.Join(root, ".agents", "skills", SkillName)
		return SkillInstallResult{
			Scope:     normalizedScope,
			Directory: directory,
			SkillFile: filepath.Join(directory, "SKILL.md"),
		}, nil
	case SkillScopeGlobal:
		homeDir, err := skillHomeDir()
		if err != nil {
			return SkillInstallResult{}, err
		}
		directory := filepath.Join(homeDir, ".agents", "skills", SkillName)
		claudeDirectory := filepath.Join(homeDir, ".claude", "skills", SkillName)
		return SkillInstallResult{
			Scope:           normalizedScope,
			Directory:       directory,
			SkillFile:       filepath.Join(directory, "SKILL.md"),
			ClaudeDirectory: claudeDirectory,
		}, nil
	default:
		return SkillInstallResult{}, fmt.Errorf("unsupported scope %q: must be local or global", scope)
	}
}

func createClaudeSkillLink(targetDirectory, claudeDirectory string) (string, error) {
	if _, err := os.Lstat(claudeDirectory); err == nil {
		return "", fmt.Errorf("claude skill path already exists at %s", claudeDirectory)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(claudeDirectory), 0o755); err != nil {
		return "", err
	}

	linkType, err := workspace.CreateLink(targetDirectory, claudeDirectory)
	if err != nil {
		return "", err
	}

	return linkType, nil
}

func removeClaudeSkillLink(claudeDirectory string) error {
	if _, err := os.Lstat(claudeDirectory); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if _, err := workspace.DetectLinkType(claudeDirectory); err != nil {
		if errors.Is(err, workspace.ErrNotLink) {
			return fmt.Errorf("refusing to remove non-link Claude skill path at %s", claudeDirectory)
		}
		return err
	}

	if err := workspace.RemoveLink(claudeDirectory); err != nil {
		return err
	}

	return nil
}
