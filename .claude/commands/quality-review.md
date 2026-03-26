---
description: Run docs-reviewer and code simplifier in parallel as a quality gate
allowed-tools: Agent, Skill
---

Launch both reviews in parallel:

1. **docs-reviewer**: Run Agent with subagent_type: docs-reviewer to detect documentation drift
2. **code-simplifier**: Run `/simplify` via the Skill tool to review recently changed code for reuse, quality, and efficiency

Execute both in a single message (Agent + Skill tool calls) for parallel execution.

After both complete, present a unified summary of findings.
