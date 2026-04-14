# Workspace X

<p align="center">
  <pre>
██╗    ██╗ ██████╗ ██████╗ ██╗  ██╗███████╗██████╗  █████╗  ██████╗███████╗    ██╗  ██╗
██║    ██║██╔═══██╗██╔══██╗██║ ██╔╝██╔════╝██╔══██╗██╔══██╗██╔════╝██╔════╝    ╚██╗██╔╝
██║ █╗ ██║██║   ██║██████╔╝█████╔╝ ███████╗██████╔╝███████║██║     █████╗       ╚███╔╝
██║███╗██║██║   ██║██╔══██╗██╔═██╗ ╚════██║██╔═══╝ ██╔══██║██║     ██╔══╝       ██╔██╗
╚███╔███╔╝╚██████╔╝██║  ██║██║  ██╗███████║██║     ██║  ██║╚██████╗███████╗    ██╔╝ ██╗
 ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝  ╚═╝ ╚═════╝╚══════╝    ╚═╝  ╚═╝
  </pre>
</p>

<p align="center">
  <strong>wsx</strong>
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.22+-00ADD8.svg" alt="Go 1.22+"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey" alt="Platform">
  <a href="https://github.com/wolf-jonathan/workspace-x/actions/workflows/ci.yml"><img src="https://github.com/wolf-jonathan/workspace-x/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/wolf-jonathan/workspace-x/commits/main"><img src="https://img.shields.io/github/last-commit/wolf-jonathan/workspace-x" alt="Last Commit"></a>
</p>

<h3 align="center">A simple way to give AI one clean workspace across all your repos.</h3>

<p align="center"><em>Point `wsx` at the repos you already have, and get one workspace folder that feels coherent to you and to your AI tools.</em></p>

<p align="center">
Workspace X helps you spin up focused AI workspaces without moving code around or manually re-explaining your repo layout every time.
</p>

---

## Why wsx

If you use AI to help you code, the setup friction gets old fast.

- One feature lives across three repos.
- Your AI tool only sees the folder you opened.
- You keep rebuilding ad hoc workspace folders.
- The next session starts with the same mess again.

`wsx` is for that exact workflow.

It lets you create one clean workspace directory that points at the repos you
already have, so you can:

- open one folder instead of juggling several
- give AI tools a clearer picture of the project you are actually working on
- keep your setup repeatable instead of rebuilding it every session
- share the workspace shape with teammates without baking in your personal paths

---

## Workspace Model

A Workspace X workspace contains:

- `.wsx.json` as the committed shared workspace config
- `.wsx.env` as the local machine-specific path variable file
- linked repo directories at the workspace root

Core invariants:

- `.wsx.json` stores portable `${VAR}` placeholders when available and must not
  be silently rewritten to machine-specific absolute paths
- path resolution happens at point of use, not during config load
- `.wsx.env` is local-only workspace state and must never be committed by
  generated workspaces
- `link_type` is runtime state and is detected from disk instead of being
  persisted in config
- `wsx exec` forwards argv directly and does not invoke a shell unless the
  caller explicitly does so


---

## Installation

Requirements:

- Go 1.22+

### Install the latest tagged version

```powershell
go install github.com/wolf-jonathan/workspace-x@latest
```

That installs `wsx` into your Go bin directory.

### Build from a local checkout

```powershell
git clone https://github.com/wolf-jonathan/workspace-x.git
cd wsx
go build -o wsx.exe .
```

### Run without installing

```powershell
go run . --help
```

---

## Quick Start

Create a workspace:

```powershell
mkdir C:\work\payments-debug
cd C:\work\payments-debug
wsx init
```

Define local path variables in `.wsx.env`:

```text
WORK_REPOS=C:\Users\you\src
PERSONAL_REPOS=C:\Users\you\projects
```

Add repositories:

```powershell
wsx add ${WORK_REPOS}\auth-service
wsx add ${WORK_REPOS}\payments-api
wsx add ${PERSONAL_REPOS}\frontend --as frontend
```

Inspect the workspace:

```powershell
wsx doctor
wsx list
wsx tree --depth 2
wsx grep "Auth"
```

Run one command across every linked repo:

```powershell
wsx exec -- git status --short --branch
```

---

## Command Reference

### Workspace commands

| Command | What It Does |
|---------|--------------|
| `wsx init [name]` | Creates `.wsx.json`, `.wsx.env`, and ensures `.wsx.env` is ignored |
| `wsx add <path> [--as <name>]` | Adds a linked repository to the workspace |
| `wsx remove <name>` | Removes a linked repository from config and removes only the workspace link |
| `wsx list [--json]` | Lists linked repos with stored path, resolved path, runtime link type, and health |
| `wsx doctor [--json] [--fix]` | Validates workspace health and portability |

### Git and execution commands

| Command | What It Does |
|---------|--------------|
| `wsx status [--json]` | Runs `git status --short --branch` across linked repositories |
| `wsx fetch [--json] [--parallel]` | Runs `git fetch --prune` across linked repositories |
| `wsx exec [--json] [--parallel] -- <cmd>` | Runs one argv-forwarded command across linked repositories |

`wsx exec` does not invoke a shell implicitly. If you need pipes, redirection, or
shell operators, call a shell explicitly:

```powershell
wsx exec -- git checkout main
wsx exec --parallel -- npm run lint
wsx exec -- powershell -Command "git fetch; git status"
```

### AI-facing commands

| Command | What It Does |
|---------|--------------|
| `wsx tree [--all] [--depth N]` | Shows a clean workspace tree |
| `wsx grep <pattern> [--include glob,...] [--exclude glob,...] [--context N] [--json]` | Searches linked repositories in config order |
| `wsx prompt [--copy]` | Generates an AI system prompt for the current workspace |
| `wsx agent-init [--purpose text]` | Generates synchronized `CLAUDE.md` and `AGENTS.md` files |

### Skill commands

| Command | What It Does |
|---------|--------------|
| `wsx skill-install [--scope local\|global]` | Installs the bundled `SKILL.md` |
| `wsx skill-uninstall [--scope local\|global]` | Removes the installed `wsx` skill |

---

## JSON-Oriented Workflows

Use the JSON flags when another tool or agent needs structured output:

```powershell
wsx doctor --json
wsx list --json
wsx status --json
wsx fetch --json --parallel
wsx exec --json -- go test ./...
wsx grep --json "TODO"
```

---

## Agent Usage

This repo intentionally keeps a small set of root Markdown files:

- `README.md` for public product and usage documentation
- `SKILL.md` for agent-native `wsx` guidance
- `AGENTS.md` and `CLAUDE.md` for workspace-aware agent tooling conventions

Install the bundled skill into the current repo scope:

```powershell
wsx skill-install
```

Install it globally for the current user:

```powershell
wsx skill-install --scope global
```

Remove an installed skill:

```powershell
wsx skill-uninstall
wsx skill-uninstall --scope global
```

---

## Release Distribution

Tagged releases are configured to build for:

- Windows `amd64`, `arm64`
- Linux `amd64`, `arm64`
- macOS `amd64`, `arm64`

To publish a release:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

That tag triggers GitHub Actions and GoReleaser, which runs the test suite,
builds archives for each platform, and uploads them to the GitHub release.

---

## Development

Run the full test suite:

```powershell
go test ./...
```

Run a single package:

```powershell
go test ./cmd
go test ./internal/ai
go test ./internal/workspace
go test ./internal/git
```

Use `go run . --help` or `go run . <command> --help` to confirm the current CLI
surface before updating docs.

---

## Project Layout

```text
wsx/
├── main.go
├── cmd/
├── internal/
│   ├── ai/
│   ├── git/
│   └── workspace/
├── README.md
├── SKILL.md
├── AGENTS.md
└── CLAUDE.md
```

---

## License

MIT
