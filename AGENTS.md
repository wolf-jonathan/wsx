# wsx Repository Instructions

## Purpose

This repository contains the `wsx` CLI described in
[docs/wsx-design-plan.md](docs/wsx-design-plan.md). `wsx` is not a generic file
sync tool. It is a Windows-first AI workspace manager that links existing local
repositories into one workspace directory.

## Product Rules

- Treat the design plan as the current source of truth unless the user explicitly overrides it.
- Preserve the core model: a workspace contains `.wsx.json`, `.wsx.env`, and linked repo directories.
- Keep `.wsx.json` portable. Stored paths should remain parameterized with `${VAR}` placeholders when available.
- Resolve `${VAR}` placeholders at point of use. Loading config must not silently rewrite them to absolute paths.
- `.wsx.env` is local-only workspace state and must never be committed by the workspace created by `wsx init`.
- On Windows, link creation must try symlinks first and fall back to directory junctions when permission errors occur.
- `link_type` is runtime state and must not be stored in `.wsx.json`.
- `wsx exec` must forward argv directly to process execution. Shell behavior is opt-in and explicit.
- Commands intended for AI agents should emit clean plain text and support `--json` where structured output is useful.
- Ship and maintain a first-party top-level `SKILL.md` so agent platforms can install `wsx` guidance globally or locally.
- Do not emit ANSI color when stdout is not a TTY.

## Engineering Priorities

- Windows correctness is first-class from the start.
- Favor a small dependency set and standard-library implementations where practical.
- Keep command output parseable and stable.
- Prefer predictable behavior over convenience magic.
- Avoid features that weaken portability or make AI usage ambiguous.

## Expected Project Structure

```text
wsx/
├── main.go
├── cmd/
│   ├── root.go
│   ├── init.go
│   ├── add.go
│   ├── remove.go
│   ├── list.go
│   ├── status.go
│   ├── fetch.go
│   ├── exec.go
│   ├── doctor.go
│   ├── dump.go
│   ├── tree.go
│   ├── grep.go
│   ├── prompt.go
│   └── claude.go
└── internal/
    ├── workspace/
    ├── git/
    └── ai/
```

## Command-Specific Expectations

- `wsx init` creates `.wsx.json`, `.wsx.env`, and ensures `.wsx.env` is added to the workspace `.gitignore`.
- `wsx add` supports absolute and parameterized input paths, detects circular references, and prevents name conflicts.
- `wsx list` reports live link health and runtime link type.
- `wsx doctor` must distinguish interactive TTY behavior from non-interactive agent or CI behavior.
- `wsx fetch` is the safe built-in multi-repo sync primitive. Do not replace it with implicit pull behavior.
- `wsx dump` must require a narrowing filter unless the caller explicitly opts into dumping everything.
- The top-level `SKILL.md` must stay aligned with the CLI behavior and the design doc invariants.

## Docs Expectations

- Keep `README.md` aligned with the current design plan and implementation status.
- If behavior changes, update the design doc or explicitly document the divergence.
- Document Windows behavior with concrete detail, especially around symlink and junction handling.

## Delivery And Handoff Expectations

- After implementing a feature, explain how to run it locally.
- After implementing a feature, explain how to run the relevant tests locally.
- If the implementation and tests pass, update progress notes for handoff to future AI agents in the appropriate project docs.

## Working Style

- Prefer small, reviewable steps that preserve the invariants above.
- If a requested change conflicts with the design plan, call out the conflict and either update the docs or get clarification.
