# agent-builder — Design

Maintain AI-agent artifacts (commands, skills, rules, agents, powers) in
canonical, platform-agnostic source files, and compile them into the native
formats of each target tool. One canonical source → many targets; one-way.

See `README.md` for usage and the contributor-facing `AGENTS.md`.

## The big realization

There is a real, broadly-adopted open standard for skills:
[**Agent Skills**](https://agentskills.io) (originally from Anthropic) — a folder
with `SKILL.md` (frontmatter `name` + `description` + body), consumed natively
by **all five targets** (Claude Code, opencode, Codex, Copilot, Kiro) and ~40
other tools.

Consequence: **skills are largely a solved problem.** Our canonical skill
format *is* the Agent Skills spec; the skill compiler validates, places the
folder at each target's skill directory, and rewrites `{{tool}}` tokens in the
body. The genuine translation work is in the kinds with *no* standard:
**commands**, **rules**, and **agents**. **Powers** (Kiro-specific) are adopted
verbatim from Kiro's `POWER.md` format and degrade to skills elsewhere.

| Kind | Standard? | What the compiler does |
|---|---|---|
| skill | Agent Skills | validate + place folder + rewrite `{{tool}}` |
| command | none | format translation (args, frontmatter, location) |
| rule | none | merge into target rules file, or standalone instructions/steering |
| agent | partial | project frontmatter onto each target's subagent schema |
| power | Kiro POWER.md | Kiro bundle on kiro; degrade to skill elsewhere |
| (all) | — | rewrite `{{tool}}` via the per-target invocation template |

## Architecture

```
canonical source (ab-src/)
        │  canon.LoadDir / ParseArtifact / ParseSkill / ParsePower
        ▼
   []*canon.Artifact  { Meta, Body, RawFrontmatter, IsSkill/IsPower }
        │  for each (artifact, target in artifact.Targets):
        ▼
   lookup Compiler(Kind, Target)  →  Compile(ctx, artifact) → []Output
        │  token.RenderBody rewrites tokens using the target Runtime
        ▼
   write build/<target>/<RelPath>
```

Packages:

- `internal/canon` — parse artifacts (frontmatter split, skill/power folder
  detection) + the tool registry (per-target invocation config, session
  server map).
- `internal/token` — the token grammar (`{{input}}`, `{{tool name@server …}}`,
  …), per-target `Runtime` mappers, body rendering + validation.
- `internal/compile` — the `Compiler` interface, the `(kind, target)` registry,
  `Execute` (the compile loop), shared compilers (command, skill, rule, merge),
  and per-target subpackages (`claude`, `opencode`, `codex`, `copilot`, `kiro`,
  `power`) that self-register via `init()`.
- `internal/cli` — argument parsing + subcommand dispatch.

## Compile matrix

| Kind | claude | opencode | codex | copilot | kiro |
|---|:--:|:--:|:--:|:--:|:--:|
| skill | `.claude/skills/` | `.opencode/skills/` | `.agents/skills/` | `.agents/skills/` | `.kiro/skills/` |
| command | `.claude/commands/` | `.opencode/commands/` | inline AGENTS.md | `.github/prompts/` | steering `manual` |
| rule | merge CLAUDE.md | merge AGENTS.md | merge AGENTS.md | `.github/instructions/` | steering `always` |
| agent | `.claude/agents/` | `.opencode/agents/` | — | `.github/agents/` | — |
| power | →skill | →skill | →skill | →skill | `.kiro/powers/` |

Notes:
- `claude`/`opencode` commands use `$ARGUMENTS`/`$N` (claude 0-indexed, opencode 1-indexed); copilot uses `${input:var}`.
- Rules merge into a single always-on file per target via idempotent managed
  regions: `<!-- BEGIN ab:rule <id> --> … <!-- END ab:rule <id> -->` (commands
  inlined into AGENTS.md use the `ab:command` namespace).
- Codex has no slash-command surface → commands are inlined into `AGENTS.md`
  automatically at compile time.
- Inapplicable (kind, target) combos skip gracefully (e.g. agents on codex/kiro).
- Kiro has no user agent file → agents are skipped on kiro.

## Tool references — inline `@server`, no registry

There is **no central tool registry**. Each tool use declares its MCP server
inline, and `execute_tool` is an ordinary MCP tool (nothing target-specific):

```
{{tool qdrant_search@scriptling collection="work-requests" top=5}}          ← direct
{{tool execute_tool@llmrouter name=fortix__summarise_work_request id=input}} ← dispatched via execute_tool
```

Built-in per-target invocation defaults (no config needed for the common case):
each is a generic direct-call template, with a `llmrouter` per-server override
for tools at the server root (like `execute_tool`):

- `claude` → `mcp__llmrouter__{server}__{tool}` (or `mcp__llmrouter__{tool}` for `@llmrouter`)
- `opencode` → `llmrouter_{server}__{tool}` (or `llmrouter__{tool}` for `@llmrouter`)
- `codex` / `copilot` / `kiro` → `unreachable` (tools degrade to inline markers)

An optional `ab-src/targets.yaml` overrides these per target and can specify
per-server templates. A `{{tool}}` with no `@server` and no known server is, in
an interactive run, prompted once (and the user is told to persist it as
`@server`); non-interactively it errors loudly.

## Token grammar

| Token | Meaning |
|---|---|
| `{{input}}` | all arguments |
| `{{arg:NAME}}` | a declared positional argument |
| `{{rules_file}}` | target's rules file path |
| `{{selection}}` | editor selection (Copilot) |
| `{{tool name@server k=v}}` | tool call (`@server` declares the MCP server) |
| `{{skill NAME}}` | reference to another skill |

Unknown `{{…}}` is left untouched. `{server}` / `{tool}` / `{params}` are the
holes in invocation templates (single-brace, distinct from body tokens).

## Extension points

**Add a target**: `TargetSpec` in `compile.go`; a `compile/<target>/` package
that `Register`s compilers and is blank-imported by `cli`; a `DefaultInvocation`
entry if tools apply; a `knownTargets` entry; tests + matrix.

**Add a kind**: `Kind` constant; parsing in `canon/parse.go` if folder-based; a
`Compiler` registered per supporting target; tests.

## Resolved decisions

1. **Adopt Agent Skills verbatim** as the canonical skill format — natively
   supported by all targets; nothing invented.
2. **No tool registry** — servers travel with the artifacts via inline `@server`.
3. **`execute_tool` is just a tool** — not a Claude-specific meta-tool default;
   authored explicitly as `{{tool execute_tool@llmrouter name=… }}`.
4. **One-way compile** — canonical → targets; generated output is never hand-edited.
5. **Native per-target placement** for skills (no shared dir), so each target
   reads its own convention directory.

## Status

All five kinds compile across all five targets; 86% test coverage; build/vet
green. See `README.md` for the usage matrix and `AGENTS.md` for contributor
conventions.
