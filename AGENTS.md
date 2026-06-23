# AGENTS.md

Guidance for AI coding agents working on this repository.

## What this is

`agent-builder` (binary `ab`) compiles canonical, platform-agnostic AI-agent
artifacts into the native formats of each target tool. One canonical source
compiles to many targets.

- **Source** lives in `ab-src/` and is the only thing you edit by hand.
- **Output** is generated into `build/<target>/...` and is never hand-edited.
- Compilation is **one-way** (canonical → targets).

## Artifact kinds

`command`, `skill`, `rule`, `agent`, `power`.

## Targets

`claude`, `opencode`, `codex`, `copilot`, `kiro`. Each has a `TargetSpec` in
`internal/compile/compile.go` (output dirs, arg indexing, rules file, etc.).

## Build, test, lint

This project uses [Task](https://taskfile.dev). Common tasks:

```
task              # fmt + lint + test + build (current platform)
task build        # build dist/ab
task build-all    # cross-compile all OS/arch combos into dist/
task test         # go test ./...
task lint         # go vet ./...
task fmt          # go fmt ./...  (ALWAYS run before committing)
task cover        # merged coverage report (project-wide)
task clean        # rm -rf dist
```

Plain Go also works: `go build ./...`, `go test ./...`, `go vet ./...`.

## Project layout

```
cmd/ab/              entrypoint (main only — keep it trivial)
internal/
  canon/             parse artifacts (frontmatter) + tool registry / invocation config
  token/             token grammar + per-target runtime rendering of bodies
  compile/           Compiler interface, matrix, Execute; command/skill/rule/merge compilers
    claude/          claude-specific (agent)
    opencode/        opencode-specific (agent)
    codex/           codex-specific (command, inline degrade)
    copilot/         copilot-specific (command prompt + agent)
    kiro/            kiro-specific (command/rule → steering)
    power/           power compiler (kiro bundle + skill degrade elsewhere)
  cli/               argument parsing + subcommand dispatch
ab-src/              canonical source (see below)
```

### Canonical source (`ab-src/`)

```
ab-src/
├── artifacts/
│   ├── commands/<id>.md          kind: command
│   ├── rules/<id>.md             kind: rule
│   ├── agents/<id>.md            kind: agent
│   ├── skills/<name>/SKILL.md    Agent Skills standard (folder)
│   └── powers/<name>/POWER.md    Kiro power (folder)
└── targets.yaml                  OPTIONAL per-target invocation overrides
```

There is **no tool registry file**. Tool servers are declared inline at each use
with `@server` (see Token grammar). `targets.yaml` is optional and only
overrides the built-in per-target invocation templates.

> Note: `ab-src/` is the user's own canonical source and is **gitignored** in
> this repo — it is project-specific, not part of the tool. The repo ships only
> the tool itself; create your own `ab-src/` per project.

## Token grammar

Tokens in canonical bodies are resolved per target and never appear in output:

| Token | Meaning |
|---|---|
| `{{input}}` | all arguments |
| `{{arg:NAME}}` | a declared positional argument |
| `{{rules_file}}` | target's always-on rules file (CLAUDE.md / AGENTS.md / …) |
| `{{selection}}` | editor selection (Copilot) |
| `{{tool name@server k=v}}` | tool call; `@server` declares the MCP server |
| `{{skill NAME}}` | reference to another skill |

`execute_tool` is an **ordinary MCP tool**, not a special case: dispatch a
fortix tool with `{{tool execute_tool@llmrouter name=fortix__summarise_work_request id=input}}`.

## Conventions

- **Go style**: gofmt'd, `go vet` clean. Follow the patterns in neighbouring files.
- **Comments**: doc comments on every exported identifier (godoc is the API docs).
  No inline implementation comments unless something is genuinely non-obvious.
- **No comments in code** beyond godoc, unless asked.
- **Error handling**: wrap with `%w`; return errors up, handle at the CLI boundary.
- **Tests**: aim for 80%+ per package (currently 86% overall). When adding a
  compiler or parser branch, add a test for it. Tests live in the package they
  test (`package X` for unexported access, `package X_test` otherwise).

## Adding a new target

1. Add a `TargetSpec` entry in `internal/compile/compile.go`.
2. Register compilers for each kind that target supports (usually a new
   `internal/compile/<target>/` package whose `init()` calls `compile.Register`).
3. Add the target name to `knownTargets` in `internal/cli/cli.go`.
4. Blank-import the new package in `internal/cli/cli.go`.
5. Add a default invocation entry in `canon.DefaultInvocation` if tools apply.
6. Add a test in the new package; extend the matrix in `README.md`/`DESIGN.md`.

## Adding a new artifact kind

1. Add a `Kind` constant in `internal/canon/artifact.go`.
2. Add parsing if it has a special layout (folder, like skill/power) in `canon/parse.go`.
3. Write a `Compiler` and `Register` it for each target that supports it.
4. Add a test per target.

## Do not

- Commit `dist/` or generated `build/` output.
- Hand-edit generated target files — change the canonical source and recompile.
- Reintroduce a central `tools.yaml` tool registry; tool servers are inline `@server`.
