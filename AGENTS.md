# wsx Repository Instructions

## Purpose

This repository contains the `wsx` CLI. `wsx` is not a generic file sync tool.
It is a Windows-first AI workspace manager that links existing local
repositories into one workspace directory.

## Product Rules

- Treat `README.md`, CLI help output, and tests as the current source of truth unless the user explicitly overrides them.
- Preserve the core model: a workspace contains `.wsx.json` and linked repo directories.
- Store absolute paths directly in `.wsx.json`.
- `.wsx.json` is local workspace state and should be gitignored by workspaces created with `wsx init`.
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
│   ├── tree.go
│   ├── grep.go
│   ├── prompt.go
│   └── agent.go
└── internal/
    ├── workspace/
    ├── git/
    └── ai/
```

## Command-Specific Expectations

- `wsx init` creates `.wsx.json` and ensures `.wsx.json` is added to the workspace `.gitignore` without overwriting existing `.gitignore` contents.
- `wsx add` stores absolute paths, supports `--favorite <NAME>` as an input shortcut, detects circular references, and prevents name conflicts.
- `wsx list` reports live link health and runtime link type.
- `wsx doctor` validates stored absolute paths, link health, and stale generated workspace instruction files.
- `wsx fetch` is the safe built-in multi-repo sync primitive. Do not replace it with implicit pull behavior.
- `wsx tree` is the discovery command. Keep it cheap and readable.
- `wsx grep` is the narrowing command. Use it before opening files broadly.
- The top-level `SKILL.md` must stay aligned with the CLI behavior and the design doc invariants.

## Docs Expectations

- Keep `README.md` aligned with the current implementation and CLI help output.
- If behavior changes, update the public docs in the same change.
- Document Windows behavior with concrete detail, especially around symlink and junction handling.

## Delivery And Handoff Expectations

- After implementing a feature, provide a recommended git commit message.
- After implementing a feature, explain how to run it locally.
- After implementing a feature, explain how to run the relevant tests locally.
- After each implementation, update the CLI help output so `--help` reflects everything currently supported.
- If the implementation and tests pass, update any public-facing handoff notes that still exist in the repo.

## Working Style

- Prefer small, reviewable steps that preserve the invariants above.
- If a requested change conflicts with the documented product behavior, call out the conflict and either update the docs or get clarification.
