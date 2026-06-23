# agent-builder

Compile canonical, platform-agnostic AI-agent artifacts (commands, skills,
rules, agents, powers) into the native formats of each target tool. One
canonical source → many targets.

## Targets

| Target | Tool |
|---|---|
| `claude` | Claude Code |
| `opencode` | opencode |
| `codex` | OpenAI Codex |
| `copilot` | GitHub Copilot |
| `kiro` | Kiro |

## Install

```
go install ./cmd/agent-builder
```

or build locally with [Task](https://taskfile.dev):

```
task build      # → dist/ab
```

## Quickstart

Author artifacts under `ab-src/` (or any directory you choose), then:

```
agent-builder validate                 # check the canonical source (default: ./ab-src)
agent-builder compile                  # compile every artifact to every target → build/
agent-builder compile examples         # compile a different source dir
agent-builder compile examples out     # ...and a different output dir
agent-builder compile --target claude  # one target
agent-builder install                  # copy build/ into each target's config location
```

Source and output paths may be positional or flags (`--source`, `--out`). Run
`agent-builder help` for the full CLI. The repo ships a small committed `examples/` you
can try immediately: `agent-builder compile examples build/examples`.

## Canonical format

Each artifact is a Markdown file with YAML frontmatter + a body that uses
portable tokens. Skills and powers are folders (`<name>/SKILL.md`,
`<name>/POWER.md`).

### command

```markdown
---
id: wr-solve
kind: command
description: Investigate a problem and produce a solution plan.
arguments:
  - { name: input, required: true }
---
Investigate **{{input}}**. Search via {{tool qdrant_search@scriptling collection="work-requests" top=5}}.
```

### skill (Agent Skills standard)

`ab-src/artifacts/skills/<name>/SKILL.md`:

```markdown
---
name: release-notes
description: Draft release notes and changelogs. Use when preparing a release.
---
Draft entries grouped by Added / Changed / Fixed / Removed.
```

Skills follow the open [Agent Skills](https://agentskills.io) standard and are
consumed natively by all five targets.

### rule / agent / power

Same frontmatter shape (`id`, `kind`, `description`, `targets`). Rules merge
into each target's always-on file (CLAUDE.md / AGENTS.md / …) or become
standalone instruction/steering files. Agents project to each target's
subagent/chatmode format. Powers are Kiro bundles that degrade to skills on
other targets.

## Token grammar

Tokens are resolved per target and never appear in output:

| Token | Meaning |
|---|---|
| `{{input}}` | all arguments |
| `{{arg:NAME}}` | a declared positional argument |
| `{{rules_file}}` | target's rules file (CLAUDE.md, AGENTS.md, …) |
| `{{selection}}` | editor selection (Copilot) |
| `{{tool name@server k=v}}` | tool call (`@server` declares the MCP server) |
| `{{skill NAME}}` | reference to another skill |

## Tool references — inline `@server`, no registry

There is no central tool registry. Each tool use declares its MCP server inline:

```
{{tool qdrant_search@scriptling collection="work-requests" top=5}}          ← direct
{{tool execute_tool@llmrouter name=fortix__summarise_work_request id=input}} ← dispatched
```

`execute_tool` is an ordinary MCP tool. Built-in per-target invocation defaults
apply (no config needed for the common case):

- `claude` → `mcp__llmrouter__{server}__{tool}` (or `mcp__llmrouter__{tool}` for `@llmrouter`)
- `opencode` → `llmrouter_{server}__{tool}` (or `llmrouter__{tool}` for `@llmrouter`)
- `codex` / `copilot` / `kiro` → `unreachable` (tools degrade to inline markers)

Override them with an optional `ab-src/targets.yaml`:

```yaml
invocation:
  claude:
    default: 'call `mcp__llmrouter__{server}__{tool}` with {params}'
    scriptling: 'call `mcp__llmrouter__{server}__{tool}` directly with {params}'
```

If a `{{tool}}` token has no `@server` and none is known, an interactive run
prompts for the server once and suggests persisting it as `@server` in the
source. Non-interactive runs error loudly (catches typos): `agent-builder compile --non-interactive`.

## Install

`agent-builder install` copies the compiled `build/<target>/` output into each
target's config location. By default it installs to the per-target **user**
config dir (stripping the target's config prefix):

| Target | Default install root |
|---|---|
| claude | `~/.claude` (`.claude/commands/x.md` → `~/.claude/commands/x.md`) |
| opencode | `~/.config/opencode` |
| codex | `~/.codex` |
| kiro | `~/.kiro` |
| copilot | `.` (repo-scoped) |

```
agent-builder install                    # all targets → their user config dirs
agent-builder install --target claude    # one target
agent-builder install --dest ~/proj      # custom root, raw paths (no strip)
agent-builder install --force            # overwrite existing files without prompting
agent-builder install --non-interactive  # never prompt; existing files are skipped
```

Existing files are **not** overwritten blindly: in an interactive run you're
prompted `overwrite <path>? [y/N/a/q]` (yes / no / all / quit). With
`--force` everything is overwritten; with `--non-interactive` existing files are
skipped. Managed-region files (`CLAUDE.md`, `AGENTS.md`, …) are **merged** into
any existing destination so hand-written content outside the managed regions is
preserved.

## Compile matrix

| Kind | claude | opencode | codex | copilot | kiro |
|---|:--:|:--:|:--:|:--:|:--:|
| skill | ✅ | ✅ | ✅ | ✅ | ✅ |
| command | `.claude/commands/` | `.opencode/commands/` | inline AGENTS.md | `.github/prompts/` | steering `manual` |
| rule | merge CLAUDE.md | merge AGENTS.md | merge AGENTS.md | `.github/instructions/` | steering `always` |
| agent | `.claude/agents/` | `.opencode/agents/` | — | `.github/agents/` | — |
| power | →skill | →skill | →skill | →skill | `.kiro/powers/` (full bundle) |

## Development

```
task              # fmt + lint + test + build
task test         # go test ./...
task cover        # merged coverage report
task build-all    # cross-compile for windows/macOS/linux × arm64/amd64
```

See `DESIGN.md` for the architecture and `AGENTS.md` for contributor guidance.

## License

MIT — see [LICENSE](LICENSE).
