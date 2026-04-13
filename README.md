# wsx

`wsx` is a Windows-first Go CLI for building AI-friendly multi-repo workspaces.
It links existing local repositories into one workspace directory so tools like
Codex, Claude Code, and Copilot can operate across them as one coherent
codebase without copying or merging anything.

## How it works

A `wsx` workspace contains:

- `.wsx.json` as the committed shared workspace config
- `.wsx.env` as the local machine-specific path variable file
- linked repo directories created as symlinks or Windows junctions

That keeps workspace config portable for teammates while the real repositories
stay in their original locations.

## Key invariants

- `.wsx.json` stores portable `${VAR}` placeholders when available and must not be silently rewritten to machine-specific absolute paths
- path resolution happens at point of use, not during config load
- `.wsx.env` is local-only workspace state and must never be committed by generated workspaces
- Windows link type is runtime state and is detected from disk instead of being persisted in config
- `wsx exec` forwards argv directly and does not invoke a shell unless the caller explicitly does so
- AI-oriented commands emit plain text by default and support `--json` where structured output is useful

## Install

`wsx` is not packaged yet for `winget`, Homebrew, or install scripts. Today the
supported install path is from source.

Requirements:

- Go 1.22+

Build a local binary:

```powershell
git clone https://github.com/jwolf/wsx.git
cd wsx
go build -o wsx.exe .
```

Install into your Go bin directory:

```powershell
git clone https://github.com/jwolf/wsx.git
cd wsx
go install .
```

Run directly from the repo without installing:

```powershell
go run . --help
```

## Quick start

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
```

## Skill install and uninstall

`wsx` ships a first-party top-level [SKILL.md](SKILL.md) for agent-native use.

Install it into the current repo scope:

```powershell
wsx skill-install
```

That writes the bundled skill to `.agents/skills/wsx/SKILL.md` under the
current directory.

Install it globally for the current user:

```powershell
wsx skill-install --scope global
```

That writes the bundled skill to `.agents/skills/wsx/SKILL.md` under the
current user home directory.

Remove an installed skill:

```powershell
wsx skill-uninstall
wsx skill-uninstall --scope global
```

`wsx skill-uninstall` removes only the installed skill directory for that
selected scope. It never mutates the source [SKILL.md](SKILL.md) in this repo.

## Command reference

### Workspace commands

`wsx init [name]`

- Initializes a workspace in the current directory
- Creates `.wsx.json`
- Creates an empty `.wsx.env`
- Ensures `.wsx.env` is in `.gitignore`

`wsx add <path> [--as <name>]`

- Adds a linked repository to the current workspace
- Accepts absolute or parameterized paths
- Derives the workspace link name from the target directory unless `--as` is provided

`wsx remove <name>`

- Removes a linked repository from `.wsx.json`
- Removes only the workspace link
- Never mutates the target repository

`wsx list [--json]`

- Lists linked repositories in workspace config order
- Reports stored path, resolved path, runtime link type, and live status

`wsx doctor [--json] [--fix]`

- Validates workspace health and portability
- Distinguishes interactive TTY behavior from non-interactive use
- `--fix` requires an interactive terminal before prompting for unresolved variables

### Git and execution commands

`wsx status [--json]`

- Runs `git status --short --branch` across linked repositories
- Exits non-zero when any repository is dirty or unavailable

`wsx fetch [--json] [--parallel]`

- Runs `git fetch --prune` across linked repositories
- `--parallel` preserves workspace config order in emitted output

`wsx exec [--json] [--parallel] -- <cmd>`

- Runs an argv-forwarded command across linked repositories
- Does not invoke a shell implicitly
- Use an explicit shell if you need pipes, redirection, or shell operators

Example:

```powershell
wsx exec -- git checkout main
wsx exec --parallel -- npm run lint
wsx exec -- powershell -Command "git fetch; git status"
```

### AI-facing commands

`wsx tree [--all] [--depth N]`

- Shows a clean workspace tree
- Respects `.gitignore` by default
- `--all` includes ignored and default-excluded files
- Defaults to `--depth 2` to keep large workspaces readable; use `--depth 0` for unlimited traversal
- Emits `...` when a directory is truncated by the active depth limit

`wsx grep <pattern> [--include glob,...] [--exclude glob,...] [--context N] [--json]`

- Searches linked repositories in workspace config order
- Respects `.gitignore` by default
- Exits non-zero when no matches are found

`wsx dump [flags]`

- Dumps selected file contents across linked repositories
- Requires one narrowing scope unless `--all-files` is set
- Supports `--include`, `--exclude`, `--path`, `--repo`, `--dry-run`, `--format`, `--max-tokens`, and `--no-ignore`

`wsx prompt [--copy]`

- Generates an AI system prompt for the current workspace
- Includes repo summaries and a workspace tree
- `--copy` writes the exact emitted prompt to the clipboard

`wsx agent-init [--purpose text]`

- Generates synchronized workspace instruction files
- Writes `CLAUDE.md` and `AGENTS.md`
- Fails if either file already exists in the workspace root
- Imports only top-level linked-repo `CLAUDE.md` and `AGENTS.md` files

### Skill commands

`wsx skill-install [--scope local|global]`

- Installs the bundled `SKILL.md`
- `local` is the default scope

`wsx skill-uninstall [--scope local|global]`

- Removes the installed `wsx` skill from the selected scope

## JSON-oriented workflows

Use the JSON flags when another tool or agent needs structured output:

```powershell
wsx doctor --json
wsx list --json
wsx status --json
wsx fetch --json --parallel
wsx exec --json -- go test ./...
wsx grep --json "TODO"
wsx dump --format json --repo auth-contracts
```

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
