# `wsx` - AI Workspace Manager
### Design & Implementation Plan

---

## 1. Overview

`wsx` is a CLI tool that manages a **workspace directory** by symlinking other local repositories into it. The workspace becomes a single, unified directory that AI tools (Claude Code, Codex, etc.) can operate across - without moving or merging any code.

**Core idea:** Instead of manually copying files or opening multiple windows, you define a workspace once, add references to any repos you care about, and AI tools see them all as one coherent codebase.

---

## 2. Design Principles

- **Zero lock-in** - the workspace is just a folder with symlinks. If you delete `wsx`, everything still works.
- **AI-first CLI output** - every command outputs clean, parseable text. No spinners or color when piped. AI agents can call `wsx` commands directly.
- **Agent-native distribution** - ship a top-level `SKILL.md` so agents like Claude, Codex, and Copilot can install `wsx` guidance globally or locally, not just invoke the binary.
- **Local and portable** - the committed config uses variable placeholders for paths. Each user defines what those variables mean on their machine in a local `.wsx.env` file that is never committed.
- **Cross-platform, Windows first** - designed to work on Windows from day one, with automatic fallback to directory junctions when symlinks require elevated permissions.

---

## 3. How It Works

```
my-workspace/          ← workspace root (any empty directory)
  .wsx.json             ← workspace config (committed, shared with teammates)
  .wsx.env              ← local path variables (gitignored, never committed)
  auth-service/        ← symlink → resolved from ${WORK_REPOS}/auth-service
  payments-api/        ← symlink → resolved from ${WORK_REPOS}/payments-api
  frontend/            ← symlink → resolved from ${WORK_REPOS}/frontend
```

You run `wsx init` once in a directory, then `wsx add` to link any repo. AI tools opened in `my-workspace/` see all three repos as subdirectories.

---

## 4. Configuration Schema

The workspace config is split into two files with different roles:

### `.wsx.json` - committed, shared

```json
{
  "version": "1",
  "name": "payments-debug",
  "created": "2026-04-12T10:00:00Z",
  "refs": [
    {
      "name": "auth-service",
      "path": "${WORK_REPOS}/auth-service",
      "added": "2026-04-12T10:05:00Z"
    },
    {
      "name": "payments-api",
      "path": "${WORK_REPOS}/payments-api",
      "added": "2026-04-12T10:06:00Z"
    },
    {
      "name": "side-project",
      "path": "${PERSONAL_REPOS}/side-project",
      "added": "2026-04-12T10:07:00Z"
    }
  ]
}
```

### `.wsx.env` - gitignored, local only

Defines what the variables mean on this specific machine:

```
WORK_REPOS=C:\Users\Yoni\work
PERSONAL_REPOS=C:\Users\Yoni\personal
```

A teammate on a different machine has their own `.wsx.env`:

```
WORK_REPOS=C:\Users\Dana\dev
PERSONAL_REPOS=C:\Users\Dana\projects
```

Same `.wsx.json` commits and works for everyone. Nobody edits the config manually.

**Design notes:**
- `wsx init` creates both `.wsx.json` and an empty `.wsx.env`, and automatically adds `.wsx.env` to `.gitignore` (creating it if needed).
- Variables use `${VAR_NAME}` syntax and are resolved at runtime from `.wsx.env`, then from the shell environment as a fallback.
- If a path contains no variables, it is used as-is (absolute path). This is the simple single-user case.
- `wsx add` accepts both plain paths (`wsx add C:\projects\auth`) and variable-prefixed paths (`wsx add ${WORK_REPOS}\auth`). When a plain path is given, `wsx` checks if any variable in `.wsx.env` is a prefix of it and offers to parameterize it automatically:
  ```
  wsx add C:\Users\Yoni\work\auth-service

  Tip: C:\Users\Yoni\work matches your WORK_REPOS variable.
  Store as ${WORK_REPOS}/auth-service for shareability? [Y/n]
  ```
- `name` is the symlink name inside the workspace. Defaults to the directory basename.
- `.wsx.json` is designed to be committed to the workspace repo or a shared meta-repo.

---

## 5. Commands

### Core Commands

#### `wsx init [name]`
Initializes a new workspace in the current directory.

```bash
wsx init
wsx init payments-debug
```

- Creates `.wsx.json` with an empty refs list.
- Creates an empty `.wsx.env` for local variable definitions.
- Adds `.wsx.env` to `.gitignore` in the workspace root (creates `.gitignore` if it doesn't exist).
- Fails gracefully if `.wsx.json` already exists.
- `name` defaults to the current directory name.

---

#### `wsx add <path> [--as <name>]`
Adds a directory reference and creates a symlink.

```bash
wsx add ../auth-service
wsx add /projects/payments-api --as payments
```

- Resolves `<path>` to an absolute path, then checks if any variable in `.wsx.env` is a prefix and offers to parameterize it (see Section 4). The parameterized form (e.g. `${WORK_REPOS}/auth-service`) is what gets stored in `.wsx.json` - never a bare absolute path, unless the user explicitly declines parameterization.
- Creates a symlink `<workspace>/<name>` → resolved absolute `<path>`.
- Detects and rejects circular references (adding the workspace itself).
- Warns if the name conflicts with an existing ref or symlink.
- On Windows: tries symlink first, automatically falls back to a directory junction if it fails due to permissions. Notes which method was used in `wsx list`.

---

#### `wsx remove <name>`
Removes a reference and deletes its symlink.

```bash
wsx remove auth-service
```

- Removes the symlink from the workspace directory.
- Removes the entry from `.wsx.json`.
- Does NOT touch the actual referenced directory.

---

#### `wsx list`
Lists all references in the current workspace.

```bash
wsx list

  NAME             PATH                                  TYPE      STATUS
  auth-service     ${WORK_REPOS}/auth-service             symlink   ok
  payments-api     ${WORK_REPOS}/payments-api             junction  ok
  old-service      ${WORK_REPOS}/old-service              symlink   broken
```

- Checks each symlink is valid (target exists).
- `path` shows the parameterized form from `.wsx.json`; `resolved_path` shows the absolute path on this machine.
- Machine-readable with `--json` flag:
  ```bash
  wsx list --json
  ```
  ```json
  [
    {
      "name": "auth-service",
      "path": "${WORK_REPOS}/auth-service",
      "resolved_path": "C:\\Users\\Yoni\\work\\auth-service",
      "link_type": "symlink",
      "status": "ok"
    },
    {
      "name": "payments-api",
      "path": "${WORK_REPOS}/payments-api",
      "resolved_path": "C:\\Users\\Yoni\\work\\payments-api",
      "link_type": "junction",
      "status": "ok"
    }
  ]
  ```

---

#### `wsx status`
Runs `git status` across all linked repos.

```bash
wsx status

  [auth-service]  2 files changed
  [payments-api]  clean
  [frontend]      3 untracked files
```

- Groups output by repo name with clear headers.
- Exit code is non-zero if any repo has changes (useful in CI or scripts).
- `--json` flag for machine-readable output.

---

#### `wsx fetch`
Runs `git fetch` across all linked repos. Read-only and always safe.

```bash
wsx fetch

  [auth-service]  Fetched. (origin/main is 3 commits ahead)
  [payments-api]  Already up to date.
  [frontend]      Fetched. (origin/main is 1 commit ahead)
```

- Reports which branch each repo is on and how far behind the local branch is.
- `--parallel` flag to run fetches concurrently.
- `--json` for machine-readable output.

> **Why not `git pull`?** Pulling across many repos can silently cause merge conflicts, detached HEAD states, or fail mid-way due to auth issues. `wsx fetch` is always safe. If you want to pull, use `wsx exec -- git pull --ff-only` which makes the intent explicit.

---

#### `wsx exec -- <cmd>`
Runs a command across all linked repos.

```bash
wsx exec -- git fetch
wsx exec -- git checkout main
wsx exec -- npm install
wsx exec -- npm run lint
wsx exec -- git pull --ff-only
```

Output is grouped by repo name:

```
[auth-service]
  Already up to date.

[payments-api]
  Updated 3 packages.

[frontend]
  Already up to date.
```

**Execution model (important):**

`wsx exec` forwards raw argv to `exec.Command` - it does **not** invoke a shell. This means:

- ✓ Plain commands work: `git checkout main`, `npm install`, `go build ./...`
- ✗ Shell operators do **not** work: `&&`, `|`, `>`, `$VAR` expansion
- ✗ Shell built-ins do **not** work: `cd`, `echo` (as shell built-in), etc.

For shell features, explicitly invoke a shell yourself:
```bash
# Windows
wsx exec -- cmd /c "git fetch && git status"
wsx exec -- powershell -Command "git fetch; git status"

# Mac/Linux
wsx exec -- bash -c "git fetch && git status"
```

This design is intentional: argv forwarding is predictable and portable. Shell invocation is opt-in and explicit, which avoids quoting bugs and platform surprises.

- Runs sequentially by default, `--parallel` for concurrent execution.
- Exit code is non-zero if any repo's command failed.
- `--json` flag for machine-readable output including per-repo exit codes and stdout/stderr:
```json
[
  { "repo": "auth-service",  "exit_code": 0, "stdout": "...", "stderr": "" },
  { "repo": "payments-api",  "exit_code": 1, "stdout": "",    "stderr": "error: ..." }
]
```

AI-friendly usage:
```bash
wsx exec --json -- npm run lint | claude -p "Summarize all lint errors across repos"
```

---

#### `wsx doctor`
Validates the workspace and reports problems. This is the most important command for sharing workspaces - it is the first thing a new teammate runs, and the first thing an AI agent should run before starting a session.

```bash
wsx doctor

  ✓ .wsx.json found and valid
  ✓ .wsx.env found
  ✓ auth-service  symlink ok       → C:\Users\Yoni\work\auth-service
  ✓ payments-api  symlink ok       → C:\Users\Yoni\work\payments-api
  ✗ old-service   broken symlink   → C:\Users\Yoni\work\old-service (not found)
  ✓ No circular references
  ✓ No duplicate names
  ✓ No repos nested inside other linked repos
  ✓ No case-collision issues detected
```

**Unresolved variable resolution (interactive):**

When a teammate runs `wsx doctor` for the first time on a shared `.wsx.json`, any variable not defined in their `.wsx.env` is caught and resolved interactively:

```
  ✗ Unresolved variable: ${WORK_REPOS}
    Used by: auth-service, payments-api
    Enter the path on your machine: C:\Users\Dana\dev
    → Saved to .wsx.env

  ✗ Unresolved variable: ${PERSONAL_REPOS}
    Used by: side-project
    Enter the path on your machine: C:\Users\Dana\projects
    → Saved to .wsx.env

  Re-checking...
  ✓ All refs resolved and valid
```

First-time setup is a single `wsx doctor` run. No manual file editing.

**Full list of checks:**
- `.wsx.json` exists and is valid JSON matching the schema
- `.wsx.env` exists (warns if missing, offers to create)
- All variables referenced in `.wsx.json` are defined in `.wsx.env` or environment (interactive resolution if not)
- Every symlink/junction target exists on disk
- No duplicate ref names
- No ref is a parent or child of the workspace directory itself
- No two refs are nested inside each other
- No name case-collisions (e.g. `Auth` and `auth`) that would cause issues on Windows/macOS
- Each linked directory is a git repo (warns if not, does not fail)

**Interactive vs non-interactive behavior:**

`wsx doctor` detects whether stdin is a TTY at startup and switches modes automatically:

- **TTY (human at a terminal):** interactive mode. Missing variables trigger prompts, answers are saved to `.wsx.env`.
- **Non-TTY (piped, CI, AI agent):** non-interactive mode. Missing variables are reported as structured errors. The command exits non-zero. It **never prompts**, never blocks, never writes to `.wsx.env` automatically.

Use `--fix` to enable interactive resolution explicitly. `--fix` requires a TTY - if stdin is not a terminal, the command errors immediately with `"--fix requires an interactive terminal"` rather than silently blocking. This makes `--fix` safe to use in scripts that may or may not have a TTY attached:
```
wsx doctor --fix
```

`--json` flag outputs all results as structured JSON - AI agents should always pass this flag:
```json
{
  "healthy": false,
  "checks": [
    { "name": "config_valid",       "status": "ok"     },
    { "name": "auth-service_link",  "status": "ok"     },
    { "name": "var_WORK_REPOS",     "status": "error",
      "message": "WORK_REPOS is not defined in .wsx.env or environment" }
  ]
}
```
Exit code is non-zero if any check fails, regardless of output format.

---

### AI-Focused Commands

#### `wsx dump`
Outputs file contents across the workspace in a single AI-digestible block. This is a **targeted extraction tool** - it is not meant to dump entire repos (which would consume a full context window). A filter is always required.

```bash
# Dump only README files - quick architecture overview
wsx dump --include "README.md"

# Dump only API contracts and schemas
wsx dump --include "*.yaml,*.json" --exclude "package*.json,*.lock"

# Dump a specific subfolder across all repos
wsx dump --path "src/api"

# Dump one small repo entirely (e.g. a types/contracts repo)
wsx dump --repo auth-contracts

# Dump everything (opt-in, not the default)
wsx dump --all-files
```

Running `wsx dump` with no flags refuses to run and prompts you to be specific:

```
wsx dump requires a filter to avoid overwhelming AI context.
Use --include, --path, or --repo to narrow the output.
Run with --all-files to override (not recommended for large repos).

Tip: try 'wsx tree' first to see what's in your workspace.
```

**Markdown output (default):**
```
# Workspace: payments-debug

## auth-service/src/main.go
```go
package main
...
```

## payments-api/src/handler.go
```go
package handler
...
```
```

**Gitignore behavior (default: on)**

By default, `wsx dump` respects `.gitignore` rules in each linked repo. This keeps AI context clean and focused - no build artifacts, secrets, lock files, or generated code cluttering the dump.

The ignore chain applied per repo, in order:
1. Global git ignore (`~/.config/git/ignore` or `~/.gitignore_global`)
2. The repo's `.gitignore`
3. Any nested `.gitignore` files within subdirectories
4. `wsx`'s own built-in defaults (see below)

**Built-in default excludes** (always applied, even with `--no-ignore` - these are noise in any AI context):

```
.git/
.DS_Store
Thumbs.db
*.pyc, __pycache__/
*.class
```

These cannot be overridden even with `--no-ignore` - they are never meaningful to an AI. (If you have a genuine edge case, use `--include` explicitly.)

**Flag reference:**

Filter flags (at least one required unless `--all-files` is set):

| Flag | Meaning |
|---|---|
| `--include "*.go,*.ts"` | Only files matching these globs |
| `--path "src/api"` | Only files under this relative path in each repo |
| `--repo <n>` | Only files from one specific linked repo |
| `--exclude "*.lock"` | Additional excludes layered on top of gitignore rules |

Scope flags (orthogonal - each controls a different axis):

| Flag | Meaning |
|---|---|
| `--all-files` | Skip the filter requirement - dump everything |
| `--no-ignore` | Disable gitignore rule parsing. Built-in excludes (`.git/`, `.DS_Store`) still apply |

`--all-files` and `--no-ignore` are independent and composable:
```bash
wsx dump --all-files                       # everything, gitignore respected
wsx dump --all-files --no-ignore           # truly everything (except .git/)
wsx dump --include "*.go" --no-ignore      # go files including gitignored ones
```

Output flags:

| Flag | Meaning |
|---|---|
| `--format json` | Structured output: `[{ "repo", "file", "content" }]` |
| `--max-tokens <n>` | Truncate with a warning if estimated token count exceeds n |
| `--dry-run` | List matched files without printing contents |

**Implementation note:** Use the `github.com/sabhiram/go-gitignore` library for `.gitignore` parsing. It handles nested `.gitignore` files and pattern precedence correctly without reimplementing git's ignore logic from scratch.

Designed to be piped directly into an AI prompt:
```bash
wsx dump --include "README.md" | claude -p "Give me an overview of how these services interact"
wsx dump --path "src/api" | claude -p "Find inconsistencies in our API design across services"
wsx dump --repo auth-contracts --all-files | claude -p "What types are exported from this package?"
```

---

#### `wsx tree`
Outputs a clean directory tree of the whole workspace.

```bash
wsx tree

payments-debug/
├── auth-service/
│   ├── src/
│   │   └── main.go
│   └── go.mod
├── payments-api/
│   ├── src/
│   │   └── handler.go
│   └── package.json
└── frontend/
    └── src/
```

- Excludes common noise dirs by default (`node_modules`, `.git`, `dist`, `vendor`).
- `--depth <n>` to control how deep the tree goes.
- Respects `.gitignore` by default. `--all` to show everything.
- Clean output for use in AI system prompts.

---

#### `wsx grep <pattern>`
Searches for a pattern across all files in all linked repos. The most practical everyday AI command - instead of dumping files, let the AI see only the relevant lines.

```bash
wsx grep "handleAuth"
wsx grep "TODO" --include "*.go"
wsx grep "process.env" --include "*.ts,*.js"
```

Output:

```
[auth-service]  src/middleware.go:14:  func handleAuth(w http.ResponseWriter, ...
[auth-service]  src/middleware.go:42:  return handleAuth(ctx, token)
[payments-api]  src/client.ts:8:       await handleAuth(request)
```

- Respects `.gitignore` by default.
- `--include` and `--exclude` glob filters.
- `--context <n>` to show n lines of context around each match (like `grep -C`).
- `--json` for machine-readable output: `[{ "repo", "file", "line", "match" }]`
- Exit code is non-zero if no matches found (useful in scripts).

This is the go-to command for cross-repo AI queries. The result set is small and targeted - feed it to an AI with a follow-up question rather than dumping whole files:

```bash
wsx grep "refreshToken" --json | claude -p "Which services handle token refresh and are they consistent?"
```

---

#### `wsx prompt`
Generates a system prompt describing the workspace, suitable for priming an AI.

```bash
wsx prompt

You are working in a multi-repo workspace called "payments-debug".
It contains the following repositories:

- auth-service (/projects/auth-service) - Go service
- payments-api (/projects/payments-api) - Node.js service
- frontend (/projects/frontend) - React app

Directory structure:
payments-debug/
├── auth-service/ ...
├── payments-api/ ...
└── frontend/ ...
```

- Detects language/framework per repo by scanning for `go.mod`, `package.json`, `Cargo.toml`, `requirements.txt`, etc.
- `--copy` flag copies output to clipboard directly.
- Designed to be pasted as a system prompt or prepended to an AI conversation.

---

#### `wsx claude-init`
Generates a `CLAUDE.md` file in the workspace root.

```bash
wsx claude-init
```

Creates a `CLAUDE.md` that Claude Code reads automatically when opened in this directory. Contents include:

- Workspace name and purpose (prompts user if not set)
- List of all repos with their detected languages
- Workspace directory structure
- Guidance for Claude on how to navigate the symlinked layout
- Any custom notes (editable section at the bottom)

Claude Code reads `CLAUDE.md` on every session, so this keeps the AI consistently oriented across sessions.

---

#### `wsx skill-install`
Installs the bundled top-level `SKILL.md` for agent use.

```bash
wsx skill-install
wsx skill-install --scope local
wsx skill-install --scope global
```

Installs the repo's bundled `SKILL.md` into the selected agent skill location.
The installed skill teaches an AI agent:

- what `wsx` is for
- the workspace invariants it must preserve
- the safest command sequence to use first, especially `wsx doctor --json`
- how to reason about linked repos, `.wsx.json`, `.wsx.env`, and Windows junction fallback
- how to avoid unsafe assumptions such as rewriting stored placeholder paths

**Distribution goal:**

`wsx` should ship with a first-party top-level `SKILL.md` in the repo and expose
an install flow so users can place it either:

- **globally** in their agent skills directory, for reuse across many sessions
- **locally** in the current repo or workspace, when they want project-scoped behavior

Target agents include Claude, Codex, Copilot, and any other agent ecosystem that
can consume a plain Markdown `SKILL.md`.

**Design constraints:**

- The bundled skill must be a plain top-level `SKILL.md`, not a generated bundle with extra metadata.
- The bundled skill must be concise and procedural, not a duplicate of the full README.
- The skill must encode the core invariants from this design doc.
- The install flow should support only scope selection, not arbitrary output paths.
- The installed skill should remain useful even when the CLI binary is not yet on `PATH`, by explaining how to locate or invoke `wsx`.

---

#### `wsx skill-uninstall`
Removes the installed `wsx` skill from the selected scope.

```bash
wsx skill-uninstall
wsx skill-uninstall --scope local
wsx skill-uninstall --scope global
```

Removes the installed skill copy without touching the repo's source `SKILL.md`.

---

## 6. Global Reference Registry (Optional, Phase 2)

A global registry at `~/.config/wsx/registry.json` stores named paths you use frequently:

```bash
wsx register ../auth-service          # registers with basename "auth-service"
wsx register ../auth-service --as auth

wsx add auth                          # adds "auth" from registry without specifying path
wsx registry list
wsx registry remove auth
```

This is a quality-of-life feature for power users. Not in MVP.

---

## 7. Project Structure

```
wsx/
├── main.go
├── go.mod
├── go.sum
├── CLAUDE.md                  ← describes the project for Claude Code sessions
├── SKILL.md                   ← bundled agent skill shipped at repo root
├── cmd/
│   ├── root.go                ← cobra root, global flags
│   ├── init.go
│   ├── add.go
│   ├── remove.go
│   ├── list.go
│   ├── status.go
│   ├── fetch.go               ← replaces sync
│   ├── exec.go
│   ├── doctor.go
│   ├── dump.go
│   ├── tree.go
│   ├── grep.go
│   ├── prompt.go
│   └── claude.go
└── internal/
    ├── workspace/
    │   ├── workspace.go       ← config load/save (placeholders kept intact on load)
    │   ├── env.go             ← .wsx.env load/save + ResolvePath() - called at point of use only
    │   └── symlink.go         ← symlink + junction create/remove, cross-platform
    ├── git/
    │   └── git.go             ← git status, fetch wrappers
    └── ai/
        ├── dump.go            ← file traversal and formatting
        ├── grep.go            ← cross-repo search
        ├── prompt.go          ← prompt generation
        ├── claude.go          ← CLAUDE.md generation
        ├── skill.go           ← skill install and uninstall logic
        ├── detect.go          ← language/framework detection
        └── ignore.go          ← gitignore chain loader (global + repo + nested)
```

---

## 8. Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config file management |
| `github.com/fatih/color` | Colored terminal output (disabled when piped) |
| `github.com/atotto/clipboard` | `--copy` flag for `wsx prompt` |
| `github.com/sabhiram/go-gitignore` | `.gitignore` parsing for `wsx dump` and `wsx tree` |

Keep dependencies minimal. No heavy frameworks. Standard library handles most file operations.

---

## 9. Cross-Platform Symlink Strategy

| Platform | Default behavior |
|---|---|
| macOS / Linux | Symlinks work natively, no special permissions needed |
| Windows | Try symlink first; auto-fallback to directory junction if permissions fail |

**Why junctions on Windows?**

Directory junctions are a Windows-native feature that behaves identically to symlinks for directories (which is all `wsx` ever links - repos are always directories). They require no elevated permissions and no Developer Mode. They are the right default fallback.

**Implementation approach in `internal/workspace/symlink.go`:**

```go
func CreateLink(target, link string) (method string, err error) {
    err = os.Symlink(target, link)
    if err == nil {
        return "symlink", nil
    }
    if isWindowsPermissionError(err) {
        err = createJunction(target, link)
        if err == nil {
            return "junction", nil
        }
    }
    return "", err
}
```

`link_type` is **not** stored in `.wsx.json` - it is machine-specific (a Windows machine without Developer Mode uses junctions, while the same ref on macOS uses symlinks). Instead, `link_type` is determined at runtime: `wsx add` and `wsx doctor` detect the actual link type on disk, and `wsx list` reports it live. If a link is recreated on a different machine (e.g. via `wsx doctor`), the new machine's native link method is used automatically.

Shown in `wsx list`:

```
NAME             PATH                              TYPE      STATUS
auth-service     ${WORK_REPOS}/auth-service        junction  ok
payments-api     ${WORK_REPOS}/payments-api        junction  ok
```

**Developer Mode note:** If a user has Developer Mode enabled on Windows, symlinks will succeed and `wsx` will use them. If not, it silently uses junctions. Either way it works - no error, no prompt, no friction.

---

## 10. Implementation Phases

### Phase 1 - Scaffold, Data Model & Windows
**Goal:** Runnable binary on Windows. Config read/write working. Symlinks and junctions both handled.

This phase is expanded because Windows support is a first-class requirement from day one, not a follow-up.

- [ ] Init Go module (`go mod init github.com/yourname/wsx`)
- [ ] Install cobra, set up `main.go` and `cmd/root.go`
- [ ] Implement `internal/workspace/env.go` - load `.wsx.env`, resolve `${VAR}` placeholders against it and the shell environment
- [ ] Define `Config` struct in `internal/workspace/workspace.go` (paths stored with placeholders)
- [ ] Implement `LoadConfig()` - walks up from cwd to find `.wsx.json`, returns raw config with placeholders **untouched**. Implement `ResolvePath(path string) (string, error)` separately in `env.go` - resolves `${VAR}` at point of use only
- [ ] Implement `SaveConfig()`
- [x] Implement `internal/workspace/symlink.go` - try symlink, auto-fallback to directory junction on Windows permission failure, return which method was used (caller reports at runtime, not persisted to config)
- [ ] Implement `wsx init` - creates `.wsx.json`, `.wsx.env`, adds `.wsx.env` to `.gitignore`
- [ ] Write `CLAUDE.md` for the project itself
- [ ] Add a first-party top-level `SKILL.md` describing how AI agents should use `wsx`

**Prompt to use:**
> "Set up a Go CLI called `wsx` using cobra. Create `internal/workspace/env.go` that loads a `.wsx.env` file (KEY=VALUE format) and resolves `${VAR}` placeholders in strings. Create `internal/workspace/workspace.go` with a Config struct matching this schema: [paste schema]. Implement `LoadConfig()` that walks up from cwd to find `.wsx.json` and returns the raw config with `${VAR}` placeholders untouched. Implement `ResolvePath(path string) (string, error)` in `env.go` that resolves a single path string against `.wsx.env` at point of use. Implement `SaveConfig()`. Create `internal/workspace/symlink.go` that creates a symlink on Mac/Linux, and on Windows tries symlink first then falls back to a directory junction silently if permissions fail. Then implement `cmd/init.go`."

---

### Phase 2 - Core Commands
**Goal:** `add`, `remove`, `list` working and tested on Windows.

- [ ] Implement `wsx add` - path resolution, variable auto-parameterize prompt, symlink/junction creation
- [ ] Implement `wsx remove`
- [ ] Implement `wsx list` with status column, link type (symlink/junction), and `--json` flag
- [ ] Add circular reference detection
- [ ] Add name conflict detection

**Prompt to use:**
> "Implement `cmd/add.go` for the `wsx` CLI. It should: resolve the given path, check if any `.wsx.env` variable is a prefix and offer to parameterize it, check for circular refs and name conflicts, create a symlink/junction via `internal/workspace/symlink.go`, and save to `.wsx.json`. Then implement `cmd/remove.go` and `cmd/list.go` with `--json` support."

---

### Phase 3 - Doctor & Portability
**Goal:** `wsx doctor` is fully working including interactive variable resolution for new teammates.

- [ ] Implement all health checks (broken refs, duplicates, nesting, case collisions, non-git dirs)
- [ ] Implement TTY-sensitive variable resolution - interactive prompt + save to `.wsx.env` when TTY, structured error when non-TTY
- [ ] Implement `--fix` flag - enables interactive resolution, errors if stdin is not a TTY
- [ ] `--json` output for AI agent consumption
- [ ] Test the full new-teammate flow end to end on Windows

**Prompt to use:**
> "Implement `cmd/doctor.go` for the `wsx` CLI. It should run all health checks listed in the design doc. At startup, detect whether stdin is a TTY. In TTY mode: for any `${VAR}` in `.wsx.json` not present in `.wsx.env` or the environment, interactively prompt the user for the value and save it to `.wsx.env`. In non-TTY mode: report unresolved variables as structured errors and exit non-zero - never prompt, never block, never write to `.wsx.env`. Add a `--fix` flag that enables interactive resolution explicitly but errors with `--fix requires an interactive terminal` if stdin is not a TTY. Output results as plain text by default and JSON with `--json`. Exit code non-zero if any check fails."

---

### Phase 4 - Git & Exec Commands
**Goal:** `status`, `fetch`, `exec`.

- [ ] Implement `internal/git/git.go` with `Status(path)` and `Fetch(path)` using `os/exec`
- [ ] Implement `wsx status` with `--json`
- [ ] Implement `wsx fetch` with `--parallel`
- [ ] Implement `wsx exec -- <cmd>` with `--parallel` and `--json`

---

### Phase 5 - AI Commands
**Goal:** `tree`, `grep`, `dump`, `prompt`, `claude-init`, `skill-install`, `skill-uninstall`.

- [ ] Implement `internal/ai/detect.go` - language/framework detection per repo
- [ ] Add `github.com/sabhiram/go-gitignore` dependency
- [ ] Implement `internal/ai/ignore.go` - gitignore chain loader (global + repo + nested)
- [ ] Implement `wsx tree` - respects gitignore by default, `--all` to bypass
- [ ] Implement `wsx grep` - cross-repo search with `--include`, `--exclude`, `--context`, `--json`
- [ ] Implement `wsx dump` - mandatory filter, gitignore support, all flags
- [ ] Implement `wsx prompt`
- [ ] Implement `wsx claude-init`
- [ ] Implement `wsx skill-install` to install the bundled top-level `SKILL.md` locally or globally
- [ ] Implement `wsx skill-uninstall` to remove the installed skill from local or global scope

---

### Phase 6 - Distribution
**Goal:** Anyone can install it.

- [ ] Write `README.md` with install instructions and full command reference
- [ ] Set up GitHub Actions to build binaries for Windows (amd64, arm64), macOS, and Linux on tag push
- [ ] Publish Windows installer via `winget` or `scoop` (priority given Windows-first audience)
- [ ] Add to Homebrew (create a tap: `homebrew-wsx`)
- [ ] Publish to pkg.go.dev
- [ ] Add install script: `curl -fsSL https://get.wsx-cli.dev | sh` (Mac/Linux) and PowerShell equivalent for Windows
- [ ] Document how to install and uninstall the bundled `wsx` skill globally or locally for Claude, Codex, Copilot, and compatible agents

---

## 11. The CLAUDE.md for `wsx` itself

When building `wsx`, drop this at the repo root so Claude Code stays oriented:

```markdown
# wsx - AI Workspace Manager

## What this is
A Go CLI tool that manages a workspace directory by symlinking other local
repos into it. Lets AI tools operate across multiple repos without merging them.

## Project layout
- cmd/         One file per CLI command, using cobra
- internal/workspace/  Config schema, load/save, symlink operations
- internal/git/        Git status and pull wrappers
- internal/ai/         Dump, prompt, and CLAUDE.md generation logic

## Config file
.wsx.json in the workspace root. See internal/workspace/workspace.go for struct.

## Key rules
- .wsx.json ALWAYS stores ${VAR} placeholders, never resolved absolute paths - this is an invariant
- LoadConfig() returns raw config with placeholders untouched; ResolvePath() resolves at point of use
- .wsx.env is always gitignored, never committed
- Symlinks (or junctions on Windows) live directly in the workspace root
- On Windows: try symlink first, silently fall back to directory junction; link_type is NOT stored in .wsx.json (it's machine-specific), detected at runtime instead
- wsx exec uses argv forwarding (exec.Command), NOT shell invocation - document this clearly
- wsx doctor: interactive only when stdin is a TTY; non-interactive + exit non-zero otherwise
- --json flag must be supported on every command that produces list output
- Never use color codes when stdout is not a TTY
```

---

## 12. Key Design Decisions Log

| Decision | Choice | Reason |
|---|---|---|
| Language | Go | Simpler syntax for vibe coding, single binary, great AI codegen support |
| CLI framework | cobra | Industry standard, well-known to AI models |
| Config format | JSON | Human-readable, no extra deps, easy to parse in scripts |
| Config location | Workspace root | Local-first, committable, shareable with teammates |
| Path portability | `${VAR}` placeholders in `.wsx.json`, resolved from `.wsx.env` | Shareable config without hardcoded machine paths |
| Local env file | `.wsx.env` gitignored | Keeps secrets and local paths out of version control |
| Symlink strategy | Symlink with auto-fallback to junction on Windows; link_type detected at runtime, not stored in config | Transparent to all tools, zero lock-in, no permissions drama; config stays portable across platforms |
| Windows support | First-class from Phase 1 | Primary developer platform; junction fallback makes it seamless |
| `wsx dump` default | Requires a filter flag | Full repo dumps destroy AI context; targeted extraction is always more useful |
| `wsx sync` → removed | Replaced by `wsx fetch` + `wsx exec` | `pull` across repos is dangerous; `exec` is more powerful and explicit |
| Output | Plain text + `--json` | AI-friendly, pipeable, no color when not TTY |
| Agent integration | Bundle a first-party top-level `SKILL.md` plus install and uninstall flow | Makes `wsx` usable as both a CLI and an agent-native capability across Claude, Codex, Copilot, and similar tools without extra metadata formats |
