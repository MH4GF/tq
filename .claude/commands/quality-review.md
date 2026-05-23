---
description: Run docs-reviewer, code-review, and diff-scoped E2E in parallel as a quality gate
allowed-tools: Agent, Skill, Bash(.claude/scripts/record-quality-review.sh)
---

Launch all three reviews in parallel:

1. **docs-reviewer**: Run Agent with `subagent_type: "docs-reviewer"` to detect documentation drift
2. **code-review**: Run Skill tool with `skill: "code-review"` to review recently changed code for reuse, quality, and efficiency
3. **e2e-execute**: Run Agent with `subagent_type: "e2e-execute"` to run diff-scoped E2E against the built tq binary

Execute all three in a single message (Agent + Skill + Agent tool calls) for parallel execution.

After all complete, present a unified summary of findings.

## Record completion

Record the current HEAD SHA **only when all three pass** the gate:

- docs-reviewer: drift acknowledged or none found
- code-review: findings acknowledged or none found
- e2e-execute: result is `PASS` or legitimate `SKIPPED` (NOT `FAIL`)

If e2e-execute returned `FAIL`, do NOT record. Fix the regression first and re-run `/quality-review`.

The recorder requires `flock` on `PATH` (macOS: `brew install flock`).

```bash
.claude/scripts/record-quality-review.sh
```

If the user makes additional commits after this, the SHA will no longer match and `/quality-review` must be re-run before `gh pr create`.
