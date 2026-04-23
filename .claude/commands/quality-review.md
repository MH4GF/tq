---
description: Run docs-reviewer and code simplifier in parallel as a quality gate
allowed-tools: Agent, Skill, Bash(.claude/scripts/record-quality-review.sh)
---

Launch both reviews in parallel:

1. **docs-reviewer**: Run Agent with `subagent_type: "docs-reviewer"` to detect documentation drift
2. **simplify**: Run Skill tool with `skill: "simplify"` to review recently changed code for reuse, quality, and efficiency

Execute both in a single message (Agent + Skill tool calls) for parallel execution.

After both complete, present a unified summary of findings.

## Record completion

After both reviews finish AND the user has acknowledged or resolved findings, record the current HEAD SHA so the `gh pr create` PreToolUse hook permits PR creation:

```bash
.claude/scripts/record-quality-review.sh
```

If the user makes additional commits after this, the SHA will no longer match and `/quality-review` must be re-run before `gh pr create`.
