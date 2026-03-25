---
description: Run docs-reviewer and code simplifier in parallel as a quality gate
allowed-tools: Agent
---

Launch both reviews in parallel using the Agent tool:

1. **docs-reviewer**: Run subagent_type: docs-reviewer to detect documentation drift
2. **code-simplifier**: Run subagent_type: code-simplifier to review recently changed code for reuse, quality, and efficiency

Execute both in a single message with two Agent tool calls (one per subagent_type) for parallel execution.

After both complete, present a unified summary of findings.
