# wsx

`wsx` is a Windows-first Go CLI for building AI-friendly multi-repo workspaces.
It links existing local repositories into a single workspace directory so tools
like Codex and Claude Code can operate across them as one coherent codebase,
without copying or merging anything.

## Status

This project is in early implementation. The shared workspace layer, Windows
link handling, `wsx init`, `wsx add`, `wsx remove`, `wsx list`, `wsx status`,
`wsx fetch`, `wsx exec`, `wsx tree`, `wsx grep`, `wsx dump`, and `wsx agent-init` are in place, and the initial
`internal/ai` gitignore and framework-detection seams are implemented for the
next AI-facing command wave.
The product direction remains defined in
[docs/wsx-design-plan.md](docs/wsx-design-plan.md).

## Core Idea

`wsx` manages a workspace folder containing:

- `.wsx.json` as the committed shared workspace config
- `.wsx.env` as the local machine-specific path variable file
- symlinks or directory junctions pointing at real repos elsewhere on disk

That makes the workspace portable for teammates while keeping the underlying
repos independent.

## Design Priorities

- Windows first, with automatic junction fallback when symlinks need elevation
- AI-friendly output with plain text by default and JSON where structured output matters
- Agent-native distribution with a top-level `SKILL.md` plus simple install and uninstall flow for Claude, Codex, Copilot, and similar tools
- Zero lock-in because the workspace is only a folder plus links
- Portable config using `${VAR}` placeholders resolved from local `.wsx.env`
- Safe multi-repo operations, favoring `fetch` over implicit `pull`

## Planned Commands

- `wsx init` to create `.wsx.json`, `.wsx.env`, and update `.gitignore`
- `wsx add` and `wsx remove` to manage linked repos
- `wsx list` and `wsx doctor` to inspect workspace health
- `wsx status`, `wsx fetch`, and `wsx exec` for git and command orchestration
- `wsx tree`, `wsx grep`, `wsx dump`, `wsx prompt`, and `wsx agent-init` for AI workflows
- `wsx skill-install` and `wsx skill-uninstall` to manage the bundled agent `SKILL.md`

## Planned Project Layout

```text
wsx/
├── main.go
├── go.mod
├── go.sum
├── AGENTS.md
├── cmd/
└── internal/
```

More detailed structure and implementation phases are documented in
[docs/wsx-design-plan.md](docs/wsx-design-plan.md).

## Key Invariants

- `.wsx.json` stores parameterized paths and must not be rewritten with resolved machine-specific paths
- path resolution happens at point of use, not during config load
- `.wsx.env` is always local-only and must never be committed from a workspace
- Windows link type is machine-specific and should be detected at runtime, not persisted in config
- `wsx exec` forwards argv directly and does not invoke a shell unless the user explicitly does so
- commands intended for AI consumption should support `--json` where list or structured output is produced
- `wsx` should ship with a first-party top-level `SKILL.md` so agents can learn the workspace rules without relying only on ad hoc prompts

## Next Implementation Target

Parallel Track C from the implementation plan:

- build `prompt` on top of the detection and tree layers

## Development

When implementation starts, this repo should remain focused on:

- minimal dependencies
- clean CLI output
- cross-platform correctness with Windows as the baseline
- testable internals for workspace, git, and AI-oriented helpers
