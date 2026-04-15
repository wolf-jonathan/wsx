---
name: wsx
description: Windows-first AI workspace manager for linked local repositories. Use only when the user explicitly asks to create or manage a wsx workspace, or when you have confirmed the current folder is already a wsx workspace and need wsx-specific inspection, health checks, multi-repo execution, or agent instruction setup. Prefer tree for discovery, grep for narrowing, and exact-file reads instead of broad content extraction.
---

# Workspace X (wsx)

Use Workspace X through the `wsx` command only for a confirmed Workspace X
workspace built from links to existing local repositories, or when the user
explicitly asks you to create one. Do not invoke `wsx` for ordinary repository
work outside a confirmed `wsx` workspace. This tool is Windows-first and is
designed for AI agents as well as humans.

## When To Use

Use `wsx` only when one of these is true:

- The user explicitly asks to create or manage a Workspace X workspace
- You have confirmed the current directory is already a `wsx` workspace by the
  presence of `.wsx.json`

Only after that confirmation should you use the other `wsx` features such as
health checks, multi-repo execution, tree discovery, or agent instruction
setup.

## Core Model

A Workspace X workspace contains:

- A local workspace config file: `.wsx.json`
- One linked directory per configured repository at the workspace root

Treat these as product invariants:

- Store absolute paths directly in `.wsx.json`.
- Treat `.wsx.json` as local workspace state. `wsx init` should ensure it is
  gitignored without overwriting an existing `.gitignore`.
- Treat `link_type` as runtime state. Detect it from disk; do not store it in
  `.wsx.json`.
- On Windows, link creation should try symlinks first and fall back to
  directory junctions on permission errors.
- `wsx exec` forwards argv directly. Shell behavior exists only if the caller
  explicitly invokes a shell such as `powershell -Command`.

## Operating Principles

- Prefer parseable output. If another tool or agent will consume the result, use
  `--json` when the command supports it.
- Prefer narrow inspection over broad extraction.
- Position `tree` as the default discovery command.
- Position `grep` as the default narrowing command after discovery.
- Respect `.gitignore` behavior by default. Only use `--no-ignore` when ignored
  files are the explicit target.

## Token Discipline

This section applies only after `wsx` is confirmed to be the right tool. Follow
it strictly.

### Preferred inspection order

1. Run `wsx doctor --json` in an unfamiliar workspace.
2. Run `wsx list --json` to understand linked repos and resolved paths.
3. Run `wsx tree` to see shape and directory layout.
4. Run `wsx grep` to find the exact files or symbols you need.
5. Read exact files directly after `grep` has narrowed the target.
6. Run `wsx prompt` only when the user explicitly needs a reusable system prompt
   for another agent.

### What counts as wasteful

- Reading broad swaths of files to "see what is here" instead of using `tree`
  first
- Running wide searches without `--include` or `--exclude` when the question is
  already scoped
- Using `--all` on `tree` or unconstrained file reads without a specific reason
- Using `--no-ignore` without a specific reason
- Emitting markdown when structured JSON would be easier for a downstream tool
- Running `wsx exec` with shell syntax but forgetting to invoke a shell

### Good patterns

- Use `wsx tree --depth 2` for quick structure
- Use `wsx grep "SymbolName" --json` to locate precise files before extraction
- Use `wsx grep "openapi" --include "*.yaml,*.json"` before opening schema files
- Read only the exact files identified by `tree` and `grep`
- Keep the number of opened files proportional to the question being asked

### Bad patterns

- Reading a whole repo when `tree` or `grep` would identify the relevant files
- `wsx prompt` when a short ad hoc explanation would do
- `wsx exec -- git status | cat` because `exec` does not implicitly use a shell

## Recommended Agent Workflow

For most work inside a `wsx` workspace:

1. `wsx doctor --json`
2. `wsx list --json`
3. `wsx tree --depth 2`
4. `wsx grep ...` or `wsx status --json`
5. Read exact files only after `tree` and `grep` have narrowed the target

For automation:

- Prefer `doctor --json`, `list --json`, `status --json`, `fetch --json`,
  `exec --json`, and `grep --json`

For setup:

- Use `agent-init` when generating workspace instruction files
- Use `favorite add`, `favorite list`, and `favorite remove` when managing
  reusable global path aliases
- Use `skill-install` or `skill-uninstall` when managing the bundled `wsx`
  skill

## Command Reference

### `wsx init [name]`

Creates a new workspace root with `.wsx.json`.

Use it when:

- Starting a new `wsx` workspace
- Creating the local workspace config scaffold

Expectations:

- Ensures `.wsx.json` is added to the workspace `.gitignore`
- Preserves the workspace model defined above

### `wsx add <path> [--as name]`

Adds an existing local repository into the workspace config and creates the
runtime link at the workspace root.

Use it when:

- Linking an existing repo into the workspace
- Adding a repo with a different visible workspace name via `--as`

Expectations:

- Accepts absolute paths and supports `--favorite <NAME>` as an input shortcut
- Also supports `--favorite <NAME>` for adding a saved global favorite directly
- Rejects circular references and name conflicts
- Stores the resolved absolute path in `.wsx.json`

### `wsx remove <name>`

Removes the workspace link and config entry for one linked repo.

Use it when:

- Detaching a repo from the workspace

Expectation:

- It must not modify or delete the target repository itself

### `wsx list [--json]`

Lists linked repos, resolved paths, and runtime link state.

Use it when:

- You need a reliable inventory of the workspace
- You want structured repo metadata before running other commands

Agent guidance:

- Prefer `--json` for automation and downstream tooling

### `wsx doctor [--json]`

Validates workspace health and portability.

Use it when:

- Entering an unfamiliar workspace
- Checking for invalid stored paths, broken links, config problems, or stale
  generated workspace instruction files

Behavior:

- It also warns when generated workspace `AGENTS.md` or `CLAUDE.md` files are
  missing or stale relative to the current workspace state

Agent guidance:

- Always prefer `wsx doctor --json`

### `wsx status [--json] [--parallel]`

Runs `git status --short --branch` across linked repositories.

Use it when:

- You need to see which repos are dirty, detached, ahead, behind, or
  unavailable

Agent guidance:

- Prefer `--json` when another tool will consume the result
- Use `--parallel` when you want faster multi-repo status checks while keeping
  workspace output order stable

### `wsx fetch [--json] [--parallel]`

Runs `git fetch --prune` across linked repositories.

Use it when:

- Refreshing repo remotes safely across the workspace

Agent guidance:

- Prefer this over inventing custom multi-repo fetch loops
- Use `--parallel` only when concurrency helps and ordered human output is not
  the main concern

### `wsx exec [--json] [--parallel] -- <cmd>`

Runs one argv-forwarded command across linked repositories.

Use it when:

- You need the same command run in each linked repo

Critical rule:

- `wsx exec` does not invoke a shell implicitly

Examples:

```powershell
wsx exec -- git checkout main
wsx exec --parallel -- npm run lint
wsx exec -- powershell -Command "git fetch; git status"
```

Agent guidance:

- Use `--json` for machine-readable output
- If you need pipes, redirection, or shell operators, explicitly invoke
  `powershell -Command`

### `wsx tree [--all] [--depth N]`

Shows a workspace tree across linked repos.

Use it when:

- You need cheap structure discovery before content extraction
- You need to compare folder layout across repos

Agent guidance:

- This is the default workspace discovery command
- The default depth is intentionally shallow and usually sufficient
- Use `--all` only when ignored files are relevant

### `wsx grep <pattern> [--include glob,...] [--exclude glob,...] [--context N] [--json]`

Searches across linked repositories in workspace config order.

Use it when:

- Locating files, symbols, text fragments, TODOs, or config keys
- Narrowing the exact files you should open next

Agent guidance:

- This is the default narrowing command after `tree`
- Use `--include` and `--exclude` aggressively to narrow scope
- Use `--json` when a tool or agent will post-process the results

### `wsx prompt [--copy]`

Generates an AI system prompt for the current workspace.

Use it when:

- The user wants a reusable prompt to hand another agent or model
- A fresh agent needs compact workspace orientation

Agent guidance:

- Do not use this by default. It is for prompt generation, not ordinary
  inspection.
- Use `--copy` only when copying to the clipboard is the actual goal

### `wsx agent-init [--purpose text]`

Generates synchronized `CLAUDE.md` and `AGENTS.md` files for the workspace.

Use it when:

- Bootstrapping agent instructions for a workspace

Expectations:

- Overwrites either target file if it already exists at the workspace root
- Emits a warning when existing files are replaced
- Keeps `AGENTS.md` and `CLAUDE.md` identical in this phase
- Indexes linked-repo instruction file references instead of importing file
  contents
- Discovers recursive linked-repo `CLAUDE.md` and `AGENTS.md` files plus exact
  `.github/copilot-instructions.md`

### `wsx favorite add <path> --name <NAME>`

Saves a reusable global favorite path.

Use it when:

- You want a reusable path alias across multiple workspaces

Expectation:

- Stores the favorite in user-scoped global config, not inside the workspace

### `wsx favorite list [--json]`

Lists saved global favorites.

Use it when:

- You need to inspect which reusable path aliases are available

Agent guidance:

- Prefer `--json` when another tool will consume the result

### `wsx favorite remove <NAME>`

Removes one saved global favorite.

Use it when:

- Cleaning up or renaming a stale global path alias

### `wsx skill-install [--scope local|global]`

Installs or refreshes the bundled `wsx` `SKILL.md`.

Use it when:

- Making the `wsx` guidance available to an agent platform

Guidance:

- `local` is the default scope
- Prefer local scope unless the user explicitly wants global installation
- Re-running `skill-install` refreshes the existing `wsx` skill in place
- `global` installs the canonical skill in `~/.agents/skills/wsx`
- `global` also creates a Claude-visible link in `~/.claude/skills/wsx`
- On Windows, the Claude link uses a symlink when available and falls back to a junction on permission errors

### `wsx skill-uninstall [--scope local|global]`

Removes the bundled `wsx` skill from the selected scope.

Use it when:

- Cleaning up an installed `wsx` skill
- `global` removes both the canonical install and the Claude mirror link

## JSON-Oriented Workflows

Use these when another tool or agent needs structured output:

```powershell
wsx doctor --json
wsx list --json
wsx status --json --parallel
wsx fetch --json --parallel
wsx exec --json -- go test ./...
wsx grep --json "TODO"
```

## Design And Handoff Sources

- The product source of truth is the current `README.md`, CLI help output, and
  tests
- Keep implementation behavior, `README.md`, and this `SKILL.md` aligned
- Keep this `SKILL.md`, `README.md`, and actual CLI behavior aligned
