---
id: code-style
kind: rule
description: Baseline code-style conventions applied to every change.
order: 10
---
- Match the style of surrounding code; don't reformat untouched lines.
- Prefer composition over inheritance; no speculative abstractions.
- Public identifiers get doc comments; no inline comments unless non-obvious.
- Handle errors at boundaries; wrap with %w; never panic in library code.
- No secrets, keys, or credentials in source — not even in examples.

These augment any project-specific rules in {{rules_file}}.
