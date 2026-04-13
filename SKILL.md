---
name: wsx
description: Windows-first AI workspace manager for linked local repositories. Use when operating inside a wsx workspace or when a user needs structured inspection, health checks, targeted code extraction, multi-repo execution, or agent instruction setup across linked repos. Prefer narrow, parseable commands and avoid token-wasteful full-workspace dumps.
---

# wsx

Use `wsx` when you need to inspect or operate on a multi-repo workspace built
from links to existing local repositories. This tool is Windows-first and is
designed for AI agents as well as humans.

## Core Model

A `wsx` workspace contains:

- A portable committed config file: `.wsx.json`
- A local machine state file: `.wsx.env`
- One linked directory per configured repository at the workspace root

Treat these as product invariants:

- Keep `${VAR}` placeholders in `.wsx.json` when available. Do not rewrite them
  to machine-specific absolute paths.
- Resolve `${VAR}` placeholders only at point of use.
- Treat `.wsx.env` as local-only state. It must stay out of git in generated
  workspaces.
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
- Prefer metadata before content: start with `doctor`, `list`, `tree`, or
  `grep` before using `dump`.
- Do not use `dump` as a default discovery tool.
- Do not use `--all-files` unless the user explicitly wants a full capture or
  the target is a very small repo.
- Respect `.gitignore` behavior by default. Only use `--no-ignore` when ignored
  files are the explicit target.

## Token Discipline

This section is the main policy for AI use. Follow it strictly.

### Preferred inspection order

1. Run `wsx doctor --json` in an unfamiliar workspace.
2. Run `wsx list --json` to understand linked repos and resolved paths.
3. Run `wsx tree` to see shape and directory layout.
4. Run `wsx grep` to find the exact files or symbols you need.
5. Run `wsx dump` only after you know the smallest useful slice to extract.
6. Run `wsx prompt` only when the user explicitly needs a reusable system prompt
   for another agent.

### What counts as wasteful

- Dumping an entire workspace to "see what is here"
- Dumping an entire medium or large repo instead of first narrowing with
  `tree`, `grep`, `--path`, or `--include`
- Using `--all-files` for convenience
- Using `--no-ignore` without a specific reason
- Emitting markdown when structured JSON would be easier for a downstream tool
- Running `wsx exec` with shell syntax but forgetting to invoke a shell

### Good patterns

- Use `wsx tree --depth 2` for quick structure
- Use `wsx grep "SymbolName" --json` to locate precise files before extraction
- Use `wsx dump --path "src/api"` instead of dumping a whole service
- Use `wsx dump --include "*.md"` for architecture summaries
- Use `wsx dump --dry-run` to inspect match scope before printing contents
- Use `wsx dump --max-tokens N` when handing content directly to an AI model

### Bad patterns

- `wsx dump --all-files` in a normal product workspace
- `wsx dump --repo frontend` when you only need one folder or file type
- `wsx prompt` when a short ad hoc explanation would do
- `wsx exec -- git status | cat` because `exec` does not implicitly use a shell

## Recommended Agent Workflow

For most work inside a `wsx` workspace:

1. `wsx doctor --json`
2. `wsx list --json`
3. `wsx tree --depth 2`
4. `wsx grep ...` or `wsx status --json`
5. `wsx dump ...` only for a narrow, chosen slice

For automation:

- Prefer `doctor --json`, `list --json`, `status --json`, `fetch --json`,
  `exec --json`, and `grep --json`

For setup:

- Use `agent-init` when generating workspace instruction files
- Use `skill-install` or `skill-uninstall` when managing the bundled `wsx`
  skill

## Command Reference

### `wsx init [name]`

Creates a new workspace root with `.wsx.json` and `.wsx.env`.

Use it when:

- Starting a new `wsx` workspace
- Creating the portable config and local env scaffold

Expectations:

- Ensures `.wsx.env` is added to the workspace `.gitignore`
- Preserves the workspace model defined above

### `wsx add <path> [--as name]`

Adds an existing local repository into the workspace config and creates the
runtime link at the workspace root.

Use it when:

- Linking an existing repo into the workspace
- Adding a repo with a different visible workspace name via `--as`

Expectations:

- Accepts absolute or parameterized paths
- Rejects circular references and name conflicts
- Creates a runtime link without rewriting portable config into machine-specific
  absolute paths

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

### `wsx doctor [--json] [--fix]`

Validates workspace health and portability.

Use it when:

- Entering an unfamiliar workspace
- Checking for missing variables, broken links, config problems, or portability
  issues

Behavior:

- In a TTY, `--fix` allows interactive resolution of missing variables
- In non-interactive use, it reports errors and exits non-zero instead of
  prompting

Agent guidance:

- Always prefer `wsx doctor --json`
- Do not use `--fix` in non-interactive agent flows

### `wsx status [--json]`

Runs `git status --short --branch` across linked repositories.

Use it when:

- You need to see which repos are dirty, detached, ahead, behind, or
  unavailable

Agent guidance:

- Prefer `--json` when another tool will consume the result

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

- Prefer this before `dump`
- The default depth is intentionally shallow and usually sufficient
- Use `--all` only when ignored files are relevant

### `wsx grep <pattern> [--include glob,...] [--exclude glob,...] [--context N] [--json]`

Searches across linked repositories in workspace config order.

Use it when:

- Locating files, symbols, text fragments, TODOs, or config keys
- Narrowing a later `dump`

Agent guidance:

- Prefer this before `dump`
- Use `--include` and `--exclude` aggressively to narrow scope
- Use `--json` when a tool or agent will post-process the results

### `wsx dump [flags]`

Dumps selected file contents across linked repositories.

Use it when:

- You already know the smallest useful slice of code or docs to extract
- You need a single AI-digestible block from a narrow cross-repo selection

Do not use it when:

- You are still discovering the workspace
- You can answer the question from `tree`, `grep`, `list`, or `status`
- The only reason to use `--all-files` is convenience

Required narrowing rule:

- `wsx dump` must be narrowed with `--include`, `--path`, or `--repo`, unless
  `--all-files` is explicitly set

Primary flags:

- `--include "*.go,*.ts"` narrows by glob
- `--exclude "*.lock"` removes noisy matches
- `--path "src/api"` narrows by relative path
- `--repo <name>` narrows to one linked repo
- `--dry-run` lists matched files without content
- `--format json` emits structured file objects
- `--max-tokens <n>` truncates once the estimated token budget is reached
- `--no-ignore` disables repo `.gitignore` parsing but still skips built-in
  noise files
- `--all-files` disables the normal filter requirement

Agent guidance:

- Use `--dry-run` first if the scope might be large
- Prefer `--path` or `--include` over repo-wide dumps
- Prefer `--max-tokens` when piping into another model
- Treat `--all-files` as an exception, not a normal workflow

Examples:

```powershell
wsx dump --include "README.md"
wsx dump --path "src/api"
wsx dump --include "*.yaml,*.json" --exclude "package*.json,*.lock"
wsx dump --dry-run --repo auth-contracts --include "*.ts"
wsx dump --format json --path "internal/workspace"
```

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

- Fails if either target file already exists at the workspace root
- Imports only top-level linked-repo `CLAUDE.md` and `AGENTS.md` files

### `wsx skill-install [--scope local|global]`

Installs the bundled `wsx` `SKILL.md`.

Use it when:

- Making the `wsx` guidance available to an agent platform

Guidance:

- `local` is the default scope
- Prefer local scope unless the user explicitly wants global installation

### `wsx skill-uninstall [--scope local|global]`

Removes the bundled `wsx` skill from the selected scope.

Use it when:

- Cleaning up an installed `wsx` skill

## JSON-Oriented Workflows

Use these when another tool or agent needs structured output:

```powershell
wsx doctor --json
wsx list --json
wsx status --json
wsx fetch --json --parallel
wsx exec --json -- go test ./...
wsx grep --json "TODO"
wsx dump --format json --path "internal/ai"
```

## Design And Handoff Sources

- The product source of truth is `docs/wsx-design-plan.md`
- The implementation and handoff source of truth is
  `docs/implementation-plan.md`
- Keep this `SKILL.md`, `README.md`, and actual CLI behavior aligned
