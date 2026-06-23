---
id: search-docs
kind: command
description: Search internal documentation for a query and summarise the top hits.
arguments:
  - { name: input, required: true }
---
Search the docs for **{{input}}** using {{tool search@docs query=input top=5}},
then list the top matches with a one-line relevance note each. If nothing
matches, say so and suggest broader search terms.
