---
description: Wait for PR checks to complete and report results
argument-hint: "[PR_URL]"
allowed-tools: Bash(gh pr checks *)
---

# Wait for PR Checks

Monitor GitHub PR checks until they complete, then report the result.

## Steps

### 1. Start watcher in background

```bash
gh pr checks $ARGUMENTS --watch
```

Run with `run_in_background: true`. If `$ARGUMENTS` is empty, this watches the PR for the current branch.

### 2. Wait for completion

Do not poll. The harness posts a user message when the background process exits — wait for that signal.

### 3. Report

- All checks passed → confirm success.
- Any check failed → list each failed check by name. Suggest `/gh-ops:fix-ci $ARGUMENTS` as the next step.
