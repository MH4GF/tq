---
description: Run docs-reviewer and code simplifier in parallel as a quality gate
allowed-tools: Agent, Skill, Bash(git rev-parse:*), Bash(mkdir:*), Bash(jq:*), Bash(date:*), Bash(mv:*), Bash(test:*), Write
---

Launch both reviews in parallel:

1. **docs-reviewer**: Run Agent with `subagent_type: "docs-reviewer"` to detect documentation drift
2. **simplify**: Run Skill tool with `skill: "simplify"` to review recently changed code for reuse, quality, and efficiency

Execute both in a single message (Agent + Skill tool calls) for parallel execution.

After both complete, present a unified summary of findings.

## Record completion

After both reviews finish AND the user has acknowledged or resolved findings, record the current HEAD SHA so the `gh pr create` PreToolUse hook permits PR creation:

```bash
mkdir -p .claude/tmp
sha=$(git rev-parse HEAD)
ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
state_file=".claude/tmp/quality-review-state.json"
[ -f "$state_file" ] || echo '{"reviewed":[]}' > "$state_file"
jq --arg sha "$sha" --arg ts "$ts" '.reviewed = ((.reviewed + [{"sha":$sha,"timestamp":$ts}]) | .[-50:])' "$state_file" > "$state_file.tmp" && mv "$state_file.tmp" "$state_file"
```

If the user makes additional commits after this, the SHA will no longer match and `/quality-review` must be re-run before `gh pr create`.
