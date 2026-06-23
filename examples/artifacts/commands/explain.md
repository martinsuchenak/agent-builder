---
id: explain
kind: command
description: Explain a concept, file, or error in plain language with examples.
arguments:
  - { name: input, required: true }
---
Explain **{{input}}** clearly.

1. If `{{input}}` names a file in the repo, read it first.
2. Summarise what it does in two sentences.
3. Give a minimal usage example.
4. Note any conventions from {{rules_file}} that apply.

Keep it to the point — no preamble.
