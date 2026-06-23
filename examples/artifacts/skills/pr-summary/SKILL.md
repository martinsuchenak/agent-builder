---
name: pr-summary
description: Summarise a pull request — group changes by area and flag anything risky. Use when reviewing a PR or drafting release notes.
---
## What I do

- Inspect the current branch's diff against its base.
- Group changes by area (backend, frontend, tests, docs, config).
- Flag anything risky: missing tests, security surface, breaking changes.
- Propose a one-paragraph PR description and a conventional-commit message.

## When to use me

Use this when the user asks for a PR summary, release notes, a commit message,
or "what changed".

## Output

Lead with the riskiest change. Keep the summary under ~200 words.
