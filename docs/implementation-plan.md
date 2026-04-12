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

### Task B1 - `status`

**Owner:** Agent 8

**Files:**

- `cmd/status.go`
- `cmd/status_test.go`

**Depends on:**

- Git runner layer
- Phase 0

### Task B2 - `fetch`

**Owner:** Agent 9

**Files:**

- `cmd/fetch.go`
- `cmd/fetch_test.go`

**Depends on:**

- Git runner layer
- Phase 0

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

### Task C1 - Framework Detection

**Owner:** Agent 12

**Files:**

- `internal/ai/detect.go`
- `internal/ai/detect_test.go`

### Task C2 - `tree`

**Owner:** Agent 13

**Files:**

- `cmd/tree.go`
- `internal/ai/tree.go`
- related tests

**Depends on:**

- ignore handling

### Task C3 - `grep`

**Owner:** Agent 14

**Files:**

- `cmd/grep.go`
- `internal/ai/grep.go`
- related tests

**Depends on:**

- ignore handling

### Task C4 - `dump`

**Owner:** Agent 15

**Files:**

- `cmd/dump.go`
- `internal/ai/dump.go`
- related tests

**Depends on:**

- ignore handling

### Task C5 - `prompt`

**Owner:** Agent 16

**Files:**

- `cmd/prompt.go`
- `internal/ai/prompt.go`
- related tests

**Depends on:**

- framework detection

### Task C6 - `claude-init`

**Owner:** Agent 17

**Files:**

- `cmd/claude.go`
- `internal/ai/claude.go`
- related tests

**Depends on:**

- framework detection

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

## Best Rollout Order

1. Foundation and seams
2. Windows link layer
3. `init`
4. In parallel: `add`, `remove`, `list`
5. In parallel: git runner layer, then `status`, `fetch`, `exec`
6. In parallel: ignore handling, detection, then `tree`, `grep`, `dump`, `prompt`, `claude-init`
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
