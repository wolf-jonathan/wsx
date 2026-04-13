# wsx Implementation Plan

## Goal

Implement `wsx` in small, parallel-friendly chunks that reduce AI-agent mistakes.
The plan uses:

- strict file ownership
- TDD for every task
- frozen shared contracts before parallel work
- independent write scopes so agents do not touch each other's code

## Core Strategy

Parallelize by file ownership, not by feature name alone.

If multiple agents are allowed to work across the same shared helpers, the usual
failure mode is collision in:

- `main.go`
- `cmd/root.go`
- shared utility layers
- test fixtures
- internal interfaces that were not frozen early

The safer approach is:

1. Define the shared seams first.
2. Freeze those contracts.
3. Split work into disjoint file ownership.
4. Require each task to start with failing tests.
5. Integrate only after the owned tests are green.

## Global Rules

- Each agent owns a disjoint write set.
- Shared contracts get defined first by one agent, then treated as stable.
- Every task starts with failing tests, then minimal implementation, then cleanup.
- Avoid parallel work on `main.go`, `cmd/root.go`, and shared utility layers until the interface is frozen.
- Keep command wiring thin. Put behavior in `internal/...` where possible.
- Do not let command agents create duplicate helpers when a shared internal package should own the concern.

## Ownership Rules

- Only one agent may touch `main.go`.
- Only one agent may touch `cmd/root.go`.
- Only one agent may define the initial public APIs in `internal/workspace/workspace.go`.
- Shared contracts must be documented and frozen before parallel command work starts.

The most important contracts to freeze early are:

- config structs
- env loader API
- link API
- git runner API
- workspace repo enumeration API

## TDD Workflow For Every Task

Every implementation task should follow this sequence:

1. Write or extend black-box tests for the owned module.
2. Run only the owned test package and confirm it fails.
3. Implement the minimum code needed to get green.
4. Refactor without changing behavior.
5. Report:
   - files changed
   - tests added
   - assumptions introduced
   - any contract that must now be treated as frozen

## Recommended Implementation Order

### Phase 0 - Foundation And Seams

This phase should be done by a single agent. It creates the interfaces that all
other tasks depend on.

**Owner:** Agent 1 only

**Files:**

- `go.mod`
- `main.go`
- `cmd/root.go`
- `internal/workspace/workspace.go`
- `internal/workspace/env.go`
- shared test helpers if needed

**TDD scope:**

- config structs
- config load and save
- `.wsx.env` parse and load
- `${VAR}` resolution
- workspace discovery by walking upward from cwd

**Constraint:**

Do not implement broad command behavior here. Add only enough wiring to compile
and support later work.

**Progress update (2026-04-12):**

- Added `go.mod` with the initial Cobra dependency.
- Added minimal CLI bootstrap in `main.go` and `cmd/root.go`.
- Implemented initial workspace contracts in `internal/workspace/workspace.go`:
  - `Config`
  - `Ref`
  - `LoadedConfig`
  - `FindWorkspaceRoot(startDir string) (string, error)`
  - `LoadConfig(startDir string) (*LoadedConfig, error)`
  - `SaveConfig(root string, cfg Config) error`
- Implemented initial env contracts in `internal/workspace/env.go`:
  - `EnvVars`
  - `LoadEnv(root string) (EnvVars, error)`
  - `ResolvePath(path string, env EnvVars) (string, error)`
- Added black-box tests for:
  - config save schema
  - upward workspace discovery
  - workspace-not-found behavior
  - `.wsx.env` parsing
  - `${VAR}` resolution precedence and unresolved-variable errors

**Frozen contracts after this slice:**

- `.wsx.json` remains the persisted config file and must keep placeholder paths untouched.
- `LoadConfig` returns raw config data and must not rewrite `${VAR}` placeholders to absolute paths.
- `ResolvePath` is the point-of-use resolver and must prefer `.wsx.env` values over process environment variables.
- Workspace discovery is defined as an upward walk from the provided start directory until `.wsx.json` is found.
- `internal/workspace` owns these config and env seams; later command work should build on them instead of redefining them.

**Verification status:**

- Tests were written first, but this Codex sandbox could not execute `go test` because `go` was not available on `PATH`.
- User-side test execution surfaced one harness issue: tests using `t.Setenv` cannot also use `t.Parallel`.
- That test issue was fixed in `internal/workspace/env_test.go`.
- Required rerun on a machine with Go available:
  - `go test ./internal/workspace`
  - `go test ./...`

### Phase 1 - Windows Link Layer

This phase is isolated enough to be owned independently once Phase 0 is stable.

**Owner:** Agent 2

**Files:**

- `internal/workspace/symlink.go`
- `internal/workspace/symlink_test.go`

**TDD scope:**

- create symlink
- Windows fallback to junction on permission failure
- detect and remove links safely

**Dependency:**

- stable path and config helpers from Phase 0

**Progress update (2026-04-12):**

- Added `internal/workspace/symlink.go` with the initial link contract:
  - `CreateLink(target, link string) (string, error)`
  - `DetectLinkType(path string) (string, error)`
  - `RemoveLink(path string) error`
- Added runtime link type constants:
  - `symlink`
  - `junction`
- Implemented Windows-first behavior:
  - try `os.Symlink` first
  - fall back to a directory junction on Windows permission failures
  - keep runtime link type detection out of `.wsx.json`
- Added tests for:
  - directory link creation
  - Windows junction fallback on symlink permission failure
  - link detection and safe link removal without touching the target
  - rejection of regular directories in `RemoveLink`

**Frozen contracts after this slice:**

- `CreateLink` returns the runtime link method used on disk and does not persist it into workspace config.
- On Windows, permission-denied symlink creation must silently fall back to a junction.
- `RemoveLink` must only delete links created in the workspace and must reject plain directories.
- `DetectLinkType` is the source of truth for runtime link reporting used by later commands like `list` and `doctor`.

**Verification status:**

- `go test ./internal/workspace`
- `go test ./...`

### Phase 2 - `init` Command

This command should stay thin and rely on the workspace layer created earlier.

**Owner:** Agent 3

**Files:**

- `cmd/init.go`
- `cmd/init_test.go`

**TDD scope:**

- create `.wsx.json`
- create `.wsx.env`
- add `.wsx.env` to `.gitignore`
- fail if already initialized

**Dependency:**

- Phase 0 complete

**Progress update (2026-04-12):**

- Added `cmd/init.go` and wired `init` into the root Cobra command.
- Added black-box tests in `cmd/init_test.go` for:
  - explicit workspace name
  - defaulting the workspace name to the current directory
  - creating `.wsx.json` with an empty refs list
  - creating an empty `.wsx.env`
  - appending `.wsx.env` to `.gitignore`
  - failing cleanly when `.wsx.json` already exists
- Kept command behavior thin:
  - config creation still flows through `internal/workspace.SaveConfig`
  - `.gitignore` mutation is limited to ensuring the `.wsx.env` entry exists once
  - no path resolution or repo-link behavior was added in this slice

**Frozen contracts after this slice:**

- `wsx init [name]` initializes the current directory only.
- When `name` is omitted, the workspace name defaults to the current directory basename.
- `wsx init` creates `.wsx.json`, creates an empty `.wsx.env`, and ensures `.wsx.env` is present in `.gitignore`.
- `wsx init` fails without mutating the workspace when `.wsx.json` already exists.

**Verification status:**

- `go test ./cmd`
- `go test ./...`

## Parallel Track A - Core Workspace Commands

These tasks can run in parallel if they do not modify shared workspace helpers.

### Task A1 - `add`

**Owner:** Agent 4

**Files:**

- `cmd/add.go`
- `cmd/add_test.go`

**Depends on:**

- Phase 0
- Windows link layer

**TDD scope:**

- absolute path resolution
- optional parameterization from `.wsx.env`
- name conflict checks
- circular reference checks
- config update and link creation

**Progress update (2026-04-12):**

- Added `cmd/add.go` and wired `add` into the root Cobra command and help text.
- Added black-box tests in `cmd/add_test.go` for:
  - resolving plain absolute paths and storing a parameterized `${VAR}` form when `.wsx.env` provides a matching prefix
  - accepting parameterized input paths directly
  - honoring `--as` for the workspace link name
  - rejecting name conflicts without mutating config
  - rejecting circular references to the workspace without mutating config
- Kept command behavior thin:
  - config loading and saving continue to flow through `internal/workspace`
  - path variable expansion continues to flow through `workspace.ResolvePath`
  - link creation continues to flow through `workspace.CreateLink`

**Frozen contracts after this slice:**

- `wsx add <path> [--as <name>]` resolves the target path at point of use and stores the unresolved placeholder form in `.wsx.json` when the input is parameterized or when a `.wsx.env` prefix match is available.
- `wsx add` derives the default ref name from the target directory basename unless `--as` is provided.
- `wsx add` must reject refs that would point at the workspace root, a path inside the workspace, or a parent directory containing the workspace.
- `wsx add` must reject conflicting ref names before creating any link or mutating `.wsx.json`.

**Verification status:**

- `go test ./cmd`
- `go test ./...`

### Task A2 - `remove`

**Owner:** Agent 5

**Files:**

- `cmd/remove.go`
- `cmd/remove_test.go`

**Depends on:**

- Phase 0
- Windows link layer

**TDD scope:**

- remove config entry
- remove link only
- leave target repo untouched

**Progress update (2026-04-12):**

- Added `cmd/remove.go` and wired `remove` into the root Cobra command and help text.
- Added black-box tests in `cmd/remove_test.go` for:
  - deleting the matching config entry
  - removing the workspace link itself
  - leaving the target repository directory untouched
  - rejecting unknown refs without mutating config
- Kept command behavior thin:
  - config loading and saving continue to flow through `internal/workspace`
  - link deletion continues to flow through `workspace.RemoveLink`
  - target repository contents remain outside the command's write scope

**Frozen contracts after this slice:**

- `wsx remove <name>` removes the named ref from `.wsx.json`.
- `wsx remove` removes only the workspace link and must not delete or mutate the target repository.
- `wsx remove` must fail cleanly when the named ref does not exist.

**Verification status:**

- `go test ./cmd`
- `go test ./...`

### Task A3 - `list`

**Owner:** Agent 6

**Files:**

- `cmd/list.go`
- `cmd/list_test.go`

**Depends on:**

- Phase 0
- Windows link layer

**TDD scope:**

- text output
- `--json`
- live status and runtime link type

**Progress update (2026-04-12):**

- Added `cmd/list.go` and wired `list` into the root Cobra command and help text.
- Added black-box tests in `cmd/list_test.go` for:
  - text output with live `ok` and `broken` status reporting
  - runtime link type reporting from `internal/workspace.DetectLinkType`
  - `--json` output including `name`, stored `path`, `resolved_path`, `link_type`, and `status`
- Kept command behavior thin:
  - config loading continues to flow through `internal/workspace`
  - placeholder resolution continues to flow through `workspace.ResolvePath`
  - runtime link inspection continues to flow through `workspace.DetectLinkType`

**Frozen contracts after this slice:**

- `wsx list` reports the stored portable path from `.wsx.json`, not a rewritten absolute path.
- `wsx list --json` emits one object per ref with `name`, `path`, `resolved_path`, `link_type`, and `status`.
- `status` is reported live at runtime and must be `ok` only when the ref resolves to an existing directory and the workspace entry is still a link.
- `link_type` remains runtime-only state derived from the on-disk link and must not be persisted into `.wsx.json`.

**Verification status:**

- Pending local verification in this Codex environment:
  - `go test ./cmd`
  - `go test ./...`

## Parallel Track B - Git Execution Layer

Keep the command-runner abstraction stable before parallelizing the git-facing
commands.

### Task B0 - Git Runner Layer

**Owner:** Agent 7

**Files:**

- `internal/git/git.go`
- `internal/git/git_test.go`

**TDD scope:**

- `Status(path)`
- `Fetch(path)`
- command execution abstraction suitable for mocking

**Progress update (2026-04-12):**

- Added `internal/git/git.go` with the shared git execution seam:
  - `CommandResult`
  - `Runner`
  - `ExecRunner`
  - `Client`
  - package helpers `Status(path)` and `Fetch(path)`
- Added black-box tests in `internal/git/git_test.go` for:
  - `Status(path)` using `git status --short --branch`
  - `Fetch(path)` using `git fetch --prune`
  - propagation of captured stdout, stderr, exit code, and runner errors
- Kept the API general enough for later command work:
  - process execution goes through one shared runner abstraction
  - `status`, `fetch`, and `exec` can all build on the same command-result shape

**Frozen contracts after this slice:**

- `internal/git.Runner` is the shared command execution abstraction for git-facing and later exec-facing command work.
- `CommandResult` owns captured `stdout`, `stderr`, and `exit_code` style data for process invocations.
- `Status(path)` runs `git status --short --branch` in the target repo directory.
- `Fetch(path)` runs `git fetch --prune` in the target repo directory.

**Verification status:**

- `go test ./...`
- Note: a direct `go test ./internal/git` run hit a Windows sandbox Go build-cache access error, but the package passed as part of `go test ./...`.

### Task B1 - `status`

**Owner:** Agent 8

**Files:**

- `cmd/status.go`
- `cmd/status_test.go`

**Depends on:**

- Git runner layer
- Phase 0

**TDD scope:**

- text output grouped by repo
- `--json`
- non-zero exit when any repo is dirty

**Progress update (2026-04-12):**

- Added `cmd/status.go` and wired `status` into the root Cobra command and help text.
- Added command-level black-box tests in `cmd/status_test.go` for:
  - grouped text output with clean, dirty, and git-error summaries
  - `--json` output including `name`, stored `path`, `resolved_path`, `summary`, `clean`, `error`, and `exit_code`
  - non-zero command failure when any repo is dirty or unavailable
- Kept command behavior thin:
  - config loading continues to flow through `internal/workspace`
  - placeholder resolution continues to flow through `workspace.ResolvePath`
  - git execution continues to flow through `internal/git`

**Frozen contracts after this slice:**

- `wsx status` runs `git status --short --branch` against each resolved repo path in workspace config order.
- `wsx status` prints one summary line per repo in plain text and supports `--json` for machine-readable output.
- `wsx status --json` emits one object per ref with `name`, `path`, `resolved_path`, `summary`, `clean`, `error`, and `exit_code`.
- `wsx status` must exit non-zero when any repo is dirty or when git status cannot be collected for a repo.

**Verification status:**

- `go test ./cmd`
- `go test ./...`

### Task B2 - `fetch`

**Owner:** Agent 9

**Files:**

- `cmd/fetch.go`
- `cmd/fetch_test.go`

**Depends on:**

- Git runner layer
- Phase 0

**TDD scope:**

- text output grouped by repo
- `--json`
- non-zero exit when any repo fetch fails
- `--parallel` while preserving workspace config order in output

**Progress update (2026-04-12):**

- Added `cmd/fetch.go` and wired `fetch` into the root Cobra command and help text.
- Added command-level black-box tests in `cmd/fetch_test.go` for:
  - grouped text output with fetched, empty-output, and git-error summaries
  - `--json` output including `name`, stored `path`, `resolved_path`, `summary`, `error`, and `exit_code`
  - `--parallel` execution while preserving workspace config order in the emitted results
- Kept command behavior thin:
  - config loading continues to flow through `internal/workspace`
  - placeholder resolution continues to flow through `workspace.ResolvePath`
  - git execution continues to flow through `internal/git`

**Frozen contracts after this slice:**

- `wsx fetch` runs `git fetch --prune` against each resolved repo path in workspace config order.
- `wsx fetch` prints one summary line per repo in plain text and supports `--json` for machine-readable output.
- `wsx fetch --json` emits one object per ref with `name`, `path`, `resolved_path`, `summary`, `error`, and `exit_code`.
- `wsx fetch --parallel` may execute fetches concurrently, but output ordering must remain aligned with workspace config order.
- `wsx fetch` must exit non-zero when any repo fetch fails or when a configured path cannot be resolved.

**Verification status:**

- `go test ./cmd`
- `go test ./...`

### Task B3 - `exec`

**Owner:** Agent 10

**Files:**

- `cmd/exec.go`
- `cmd/exec_test.go`

**Depends on:**

- command runner abstraction
- Phase 0

**Important note:**

Do not let `git.go` and `exec.go` invent separate process-running models. One
shared abstraction should be frozen first.

## Parallel Track C - AI-Facing Workspace Inspection

These tasks become safe once ignore handling and framework detection are split
into separate owned modules.

### Task C0 - Ignore Handling

**Owner:** Agent 11

**Files:**

- `internal/ai/ignore.go`
- `internal/ai/ignore_test.go`

**TDD scope:**

- built-in default excludes
- global git ignore loading
- repo and nested `.gitignore` loading
- nested precedence and branch scoping

**Progress update (2026-04-12):**

- Added `internal/ai/ignore.go` with the initial ignore seam:
  - `IgnoreMatcher`
  - `LoadIgnoreMatcher(repoRoot string) (*IgnoreMatcher, error)`
  - `(*IgnoreMatcher) MatchesPath(path string) bool`
- Added `github.com/sabhiram/go-gitignore` as the planned parser dependency.
- Implemented the initial ignore chain loader for AI-facing commands:
  - loads the global git ignore from `~/.config/git/ignore` or `~/.gitignore_global` when present
  - walks the repo for `.gitignore` files outside `.git/`
  - rewrites nested `.gitignore` patterns into one repo-relative precedence chain
  - keeps built-in noise excludes enforced separately so later commands cannot accidentally surface `.git/`, `.DS_Store`, `Thumbs.db`, `*.pyc`, `__pycache__/`, or `*.class`
- Added black-box tests in `internal/ai/ignore_test.go` for:
  - repo root ignore rules
  - nested `.gitignore` negation overriding an earlier parent ignore
  - global ignore file loading from the user home directory
  - nested ignore scoping applying only within the matching repo branch

**Frozen contracts after this slice:**

- `internal/ai` now owns gitignore-chain loading for later AI-facing commands instead of duplicating ignore handling inside `tree`, `grep`, or `dump`.
- `LoadIgnoreMatcher` resolves the repo root once and returns a matcher that accepts repo-relative or absolute paths under that root.
- Built-in AI noise excludes are always enforced by the matcher and are not dependent on repository ignore files.
- Nested `.gitignore` files participate in a single ordered precedence chain so deeper rules can override broader parent rules within their branch.

**Verification status:**

- `go test ./internal/ai`
- `go test ./...`

### Task C1 - Framework Detection

**Owner:** Agent 12

**Files:**

- `internal/ai/detect.go`
- `internal/ai/detect_test.go`

**TDD scope:**

- marker-file based language detection
- common framework detection for Node and Python repos
- stable fallback when no known markers exist

**Progress update (2026-04-12):**

- Added `internal/ai/detect.go` with the initial framework detection seam:
  - `RepoDetection`
  - `DetectRepo(repoRoot string) (RepoDetection, error)`
- Implemented marker-file language detection for:
  - Go via `go.mod`
  - Node.js via `package.json`
  - Rust via `Cargo.toml`
  - Python via `pyproject.toml` or `requirements.txt`
- Implemented lightweight framework detection for common later prompt-generation use cases:
  - Node.js: `Next.js`, `Nuxt`, `SvelteKit`, `NestJS`, `Express`, `React`, `Vue`
  - Python: `FastAPI`, `Django`, `Flask`
- Added black-box tests in `internal/ai/detect_test.go` for:
  - Go module detection
  - Next.js detection from `package.json`
  - FastAPI detection from `pyproject.toml`
  - Rust detection from `Cargo.toml`
  - stable `Unknown` fallback when no recognized markers exist

**Frozen contracts after this slice:**

- `internal/ai` now owns repo language/framework detection for later `prompt` and `agent-init` work instead of scattering marker detection across commands.
- `DetectRepo` is intentionally marker-file driven and returns a compact `RepoDetection` with `language`, optional `framework`, and `indicators`.
- Unknown repos must still return a successful `RepoDetection` with `Language: "Unknown"` rather than failing.

**Verification status:**

- `go test ./internal/ai`
- `go test ./...`

### Task C2 - `tree`

**Owner:** Agent 13

**Files:**

- `cmd/tree.go`
- `internal/ai/tree.go`
- related tests

**Depends on:**

- ignore handling

**TDD scope:**

- clean workspace tree output
- default ignore-aware traversal
- `--all` bypass for ignored and default-noise paths
- `--depth` limiting per-repo traversal

**Progress update (2026-04-12):**

- Added `internal/ai/tree.go` with the initial tree-rendering seam:
  - `TreeRepo`
  - `TreeOptions`
  - `RenderWorkspaceTree(workspaceName string, repos []TreeRepo, options TreeOptions) (string, error)`
- Added `cmd/tree.go` and wired `tree` into the root Cobra command and help text.
- Implemented clean tree rendering for workspace refs:
  - preserves workspace config order for top-level repo entries
  - sorts child entries with directories before files
  - uses the shared ignore matcher by default
  - applies tree-specific noise exclusion for `node_modules`, `dist`, and `vendor` unless `--all` is set
- Added black-box tests in:
  - `internal/ai/tree_test.go` for ignore handling, `--all` behavior, and depth limiting
  - `cmd/tree_test.go` for end-to-end workspace output and command flag behavior

**Frozen contracts after this slice:**

- `internal/ai.RenderWorkspaceTree` owns tree formatting for later AI-facing prompt and inspection work instead of duplicating traversal logic inside commands.
- `wsx tree` is plain-text only in this slice and renders the workspace root followed by linked repos in workspace config order.
- `wsx tree --depth N` limits traversal depth within each linked repo; `0` means unlimited.
- `wsx tree --all` bypasses ignore-based filtering and tree-specific default noise excludes.

**Verification status:**

- `go test ./internal/ai ./cmd`
- `go test ./...`

### Task C3 - `grep`

**Owner:** Agent 14

**Files:**

- `cmd/grep.go`
- `internal/ai/grep.go`
- related tests

**Depends on:**

- ignore handling

**Progress update (2026-04-13):**

- Added `internal/ai/grep.go` with the initial search seam:
  - `GrepRepo`
  - `GrepOptions`
  - `GrepMatch`
  - `GrepWorkspace(repos, pattern, options)`
- Added `cmd/grep.go` and wired `grep` into the root Cobra command and help text.
- Implemented cross-repo text search behavior for AI-facing use:
  - preserves workspace config order by searching repos in config order
  - respects the shared ignore matcher by default
  - supports comma-separated `--include` and `--exclude` glob filters
  - supports `--context` in both plain-text and JSON output
  - skips binary files and exits non-zero when no matches are found
- Added black-box tests in:
  - `internal/ai/grep_test.go` for ignore handling, include/exclude filtering, context lines, and binary-file skipping
  - `cmd/grep_test.go` for end-to-end text output, `--json`, and no-match exit behavior

**Frozen contracts after this slice:**

- `internal/ai.GrepWorkspace` owns ignore-aware cross-repo text search for later AI-facing commands instead of duplicating traversal logic inside commands.
- `wsx grep <pattern>` searches linked repos in workspace config order and emits plain text by default.
- `wsx grep --json` emits one object per match with `repo`, `file`, `line`, `match`, and optional context arrays when requested.
- `wsx grep` respects `.gitignore` by default and supports `--include`, `--exclude`, and `--context`.
- `wsx grep` must exit non-zero when no matches are found.

**Verification status:**

- Verified on 2026-04-13 with:
  - `go test ./...`
- Follow-up note:
  - stabilized `cmd/add_test.go` on Windows by comparing resolved link targets with `os.SameFile` instead of raw path strings, because `filepath.EvalSymlinks` may return a different canonical spelling for the same directory
  - corrected the `internal/ai/grep_test.go` context fixture so it contains exactly two pattern matches, which keeps the test aligned with the documented "one result per literal matching line" contract

### Task C4 - `dump`

**Owner:** Agent 15

**Files:**

- `cmd/dump.go`
- `internal/ai/dump.go`
- related tests

**Depends on:**

- ignore handling

**Progress update (2026-04-13):**

- Added `internal/ai/dump.go` with the initial dump seam:
  - `DumpRepo`
  - `DumpOptions`
  - `DumpFile`
  - `DumpResult`
  - `DumpWorkspace(...)`
  - `RenderDumpMarkdown(...)`
  - `ValidateDumpScope(...)`
- Added `cmd/dump.go` and wired `dump` into the root Cobra command and help text.
- Implemented targeted workspace extraction behavior for AI-facing use:
  - requires one narrowing scope by default via `--include`, `--path`, or `--repo`
  - supports `--all-files` as the explicit override
  - respects the shared ignore matcher by default
  - supports `--no-ignore` while still enforcing built-in noise excludes like `.git/`
  - supports `--exclude`, `--dry-run`, `--format json`, and `--max-tokens`
  - preserves workspace config order for repos and sorted relative paths within each repo
- Added black-box tests in:
  - `internal/ai/dump_test.go` for filtering, ignore handling, dry-run behavior, markdown rendering, and token-budget truncation
  - `cmd/dump_test.go` for scope enforcement, markdown output, JSON output, repo narrowing, `--no-ignore`, and `--max-tokens`

**Frozen contracts after this slice:**

- `wsx dump` must refuse to run without `--include`, `--path`, `--repo`, or `--all-files`.
- `wsx dump` respects `.gitignore` by default and `--no-ignore` disables only repository ignore rules, not the built-in noise excludes.
- `wsx dump --format json` emits one object per matched file with `repo`, `file`, and `content` omitted during `--dry-run`.
- `wsx dump --max-tokens` truncates the emitted file list once the estimated token budget is exceeded and surfaces a warning in markdown output.

**Verification status:**

- `go test ./internal/ai -run Dump`
- `go test ./cmd -run Dump`

### Task C5 - `prompt`

**Owner:** Agent 16

**Files:**

- `cmd/prompt.go`
- `internal/ai/prompt.go`
- related tests

**Depends on:**

- framework detection

**Progress update (2026-04-13):**

- Added `internal/ai/prompt.go` with the initial prompt-rendering seam:
  - `PromptRepo`
  - `WorkspacePrompt`
  - `GenerateWorkspacePrompt(...)`
  - `RenderWorkspacePrompt(...)`
- Added `cmd/prompt.go` and wired `prompt` into the root Cobra command and help text.
- Implemented workspace prompt generation behavior for AI-facing use:
  - detects each linked repo language and framework via `internal/ai.DetectRepo`
  - renders repo summaries with resolved absolute repo roots
  - includes a shallow workspace tree for navigation context
  - supports `--copy` to place the generated prompt onto the system clipboard
- Added black-box tests in:
  - `internal/ai/prompt_test.go` for repo summaries, framework labeling, tree inclusion, and unknown fallback behavior
  - `cmd/prompt_test.go` for end-to-end command output and `--copy` clipboard behavior

**Frozen contracts after this slice:**

- `wsx prompt` emits plain text only in this slice and is designed to be pasted directly into an AI system prompt.
- Repo summaries in `wsx prompt` must include the linked repo name, resolved absolute path, detected language, and detected framework when present.
- `wsx prompt` includes a workspace tree rendered from linked repos in workspace config order.
- `wsx prompt --copy` copies exactly the emitted prompt text to the clipboard.

**Verification status:**

- `go test ./internal/ai -run Prompt`
- `go test ./cmd -run "Test(Prompt|RootHelpShowsSupportedCommands)"`

### Task C6 - `agent-init`

**Owner:** Agent 17

**Files:**

- `cmd/agent.go`
- `internal/ai/agent.go`
- related tests

**Depends on:**

- framework detection

**Progress update (2026-04-13):**

- Added `internal/ai/agent.go` with workspace-instruction generation helpers:
  - `InstructionRepo`
  - `WorkspaceInstructions`
  - `GenerateWorkspaceInstructions(...)`
  - `RenderWorkspaceInstructions(...)`
  - `WriteWorkspaceInstructionFiles(...)`
- Added `cmd/agent.go` and wired `agent-init` into the root Cobra command and help text.
- Expanded the original slice so one command now writes three synchronized top-level files:
  - `CLAUDE.md`
  - `AGENTS.md`
  - `.github/copilot-instructions.md`
- Implemented linked-repo instruction import behavior:
  - scans each resolved repo for `CLAUDE.md`, `AGENTS.md`, and `.github/copilot-instructions.md`
  - includes imported content in clearly labeled repo-specific sections
  - keeps workspace-wide rules and repo-scoped instructions distinct in the generated output
- Added black-box tests in:
  - `internal/ai/agent_test.go` for detection summaries, imported instruction rendering, and file writing
  - `cmd/agent_test.go` for end-to-end command output and generated workspace files

**Frozen contracts after this slice:**

- `wsx agent-init` generates shared workspace instruction content and writes it to `CLAUDE.md`, `AGENTS.md`, and `.github/copilot-instructions.md` in the workspace root.
- Imported linked-repo instruction files must be labeled by both repo name and source file path in the generated output.
- Repo instruction discovery for this slice includes `CLAUDE.md`, `AGENTS.md`, and `.github/copilot-instructions.md` anywhere under a linked repo, excluding `.git/`.

**Verification status:**

- Pending local verification in this Codex environment because `go` is not available on `PATH`:
  - `go test ./internal/ai ./cmd`
  - `go test ./...`

## Keep `doctor` Separate

`doctor` should be assigned to the strongest agent and kept out of the early
parallel wave.

**Owner:** strongest available agent only

**Files:**

- `cmd/doctor.go`
- `cmd/doctor_test.go`
- optionally `internal/workspace/doctor.go`

**Why it is high risk:**

- crosses config, env, links, and path validation
- includes TTY-sensitive behavior
- includes interactive and non-interactive modes
- includes machine-readable JSON output

This command is likely to break if built during active foundational refactoring.

**Progress update (2026-04-13):**

- Added `cmd/doctor.go` and wired `doctor` into the root Cobra command and help text.
- Added command-level black-box tests in `cmd/doctor_test.go` for:
  - healthy workspace validation in plain text output
  - non-interactive unresolved-variable reporting in `--json`
  - `--fix` refusing to run without a TTY
  - interactive variable resolution that writes `.wsx.env`
- Implemented the initial doctor behavior for portability and teammate onboarding:
  - validates `.wsx.json` loading and `.wsx.env` presence
  - detects unresolved `${VAR}` placeholders and prompts only when stdin is interactive
  - writes resolved variables back to `.wsx.env`
  - checks duplicate names, case-collision risks, workspace nesting, nested refs, live links, and non-git warnings

**Frozen contracts after this slice:**

- `wsx doctor` emits plain text by default and supports `--json` with a top-level `healthy` boolean and a `checks` array of `{name,status,message}` objects.
- Unresolved variables are prompted for only when stdin is interactive; non-interactive runs report them as errors and exit non-zero.
- `wsx doctor --fix` requires an interactive terminal and errors immediately otherwise.
- Missing git metadata under a linked repo is a warning, not a failing health check.

**Verification status:**

- `go test ./cmd -run "Test(Doctor|RootHelpShowsSupportedCommands)"`
- `go test ./... -run TestDoctor`

## Skill Support

Keep skill support late in the implementation sequence.

**Owner:** docs-focused or integration-focused agent

**Files:**

- `SKILL.md`
- `cmd/skill_install.go`
- `cmd/skill_uninstall.go`
- `internal/ai/skill.go`

**Recommendation:**

Implement this after the core CLI behavior is stable, so the shipped `SKILL.md`
describes real command behavior instead of speculation.

**Progress update (2026-04-13):**

- Added the bundled top-level `SKILL.md` for agent-native distribution.
- Added the repo-level `CLAUDE.md` so Claude-oriented sessions have project guidance at the source root.
- Added `internal/ai/skill.go` with the initial skill installation seam:
  - `SkillInstallResult`
  - `InstallBundledSkill(repoRoot, scope string) (SkillInstallResult, error)`
  - `UninstallBundledSkill(repoRoot, scope string) (SkillInstallResult, error)`
- Added `cmd/skill_install.go` and `cmd/skill_uninstall.go` and wired both commands into the root Cobra help output.
- Added black-box tests in:
  - `internal/ai/skill_test.go` for local install, global install, duplicate install rejection, uninstall, and missing-install behavior
  - `cmd/skill_install_test.go` for end-to-end local install, global install, and uninstall behavior

**Frozen contracts after this slice:**

- `wsx skill-install` supports only `--scope local` and `--scope global`.
- Local install writes the bundled `SKILL.md` to `.agents/skills/wsx/SKILL.md` under the current directory.
- Global install writes the bundled `SKILL.md` to `.agents/skills/wsx/SKILL.md` under the current user home directory.
- `wsx skill-uninstall` removes only the installed `wsx` skill directory for the selected scope and never mutates the source `SKILL.md`.

**Verification status:**

- Pending local verification in this Codex environment because `go` is not available on `PATH`:
  - `go test ./internal/ai`
  - `go test ./cmd`
  - `go test ./...`

## Distribution Docs

**Progress update (2026-04-13):**

- Expanded `README.md` from a status stub into an operator-facing document with:
  - source install instructions
  - quick-start workspace setup
  - skill install and uninstall instructions for local and global scope
  - a command-by-command reference aligned with current `--help` output
- Updated `docs/wsx-design-plan.md` to mark the README and skill installation documentation tasks complete.

**Verification status:**

- Verified on 2026-04-13 with:
  - `go test ./...`

## Best Rollout Order

1. Foundation and seams
2. Windows link layer
3. `init`
4. In parallel: `add`, `remove`, `list`
5. In parallel: git runner layer, then `status`, `fetch`, `exec`
6. In parallel: ignore handling, detection, then `tree`, `grep`, `dump`, `prompt`, `agent-init`
7. `doctor`
8. `SKILL.md`, `skill-install`, `skill-uninstall`
9. Full integration pass across the whole CLI

## Coordination Guidance

To keep tasks truly independent:

- assign one owner per file group
- freeze shared interfaces before parallel work begins
- require every agent to report assumptions explicitly
- reject tasks that require edits across another agent's owned files
- keep command files thin and move reusable behavior into internal packages

## Suggested Next Artifact

The next useful planning artifact is a task board that includes:

- exact agent prompts
- acceptance tests per task
- dependency graph
- file ownership table
- merge order

This document should be treated as the high-level execution strategy for that
board.
