---
name: deploy
displayName: Deploy Helper
description: Guide a safe deployment — pre-checks, rollback plan, and post-deploy verification. Activates on deploy/release/rollout keywords.
keywords: [deploy, release, rollout, ship, rollback]
author: examples
---
# Onboarding

Confirm the deploy CLI is available: run `deploy --version`. If missing, stop
and tell the user how to install it before continuing.

# Steering

- Run pre-deploy checks (tests, lint, build) and stop on any failure.
- Capture the previous version/commit so rollback is one command.
- Deploy, then verify health against the public endpoint before declaring done.
- On any verification failure, roll back immediately and report what happened.
