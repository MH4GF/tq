---
description: Periodic golden rules GC — detect Rule 7 violations and docs drift, then create tq actions for fixes
allowed-tools: Read, Glob, Grep, Bash, Agent, Skill
---

Detect violations that CI cannot catch: Rule 7 (table-driven tests) and documentation drift. Create tq actions for each finding.

IMPORTANT: If no violations found, report "No violations detected" and stop. Do NOT create actions when there is nothing to fix.

## Phase 1: Parallel scan

Run both checks simultaneously in a single message with two Agent tool calls.

### Check A: Rule 7 — Table-driven tests

Scan `*_test.go` in `cmd/`, `dispatch/`, `tui/`, `db/`.

<heuristics>
Flag as violation:
- 3+ sequential calls to the same assertion in one `func Test*`
- Multiple `t.Run` subtests sharing identical structure (copy-paste bodies)
- `if`-chain where each branch tests a different input/expected pair

NOT violations:
- Test helpers and setup functions
- Genuinely distinct scenarios using similar assertions
- Tests with ≤2 cases
</heuristics>

Record per finding: file path, function name, line range, why it should be table-driven.

### Check B: Documentation drift

Delegate to docs-reviewer agent:

```
Agent(subagent_type: "docs-reviewer")
```

Collect findings (file, section, issue, severity).

## Phase 2: Create tq actions

For each violation, run `/tq:create-action`.

<constraints>
- One action per violation (group tightly related violations in the same file)
- Include file:line and what to fix in the instruction
- Set `--meta '{"claude_args":["--permission-mode","auto","--worktree","<scope-name>"]}'` where `<scope-name>` is derived from the violation's scope (e.g., `rule7-cmd-foo-test`, `docs-cli-reference`)
- Use the same task as this GC action (check session context)
</constraints>

<example>
Rule 7 instruction:
```
Refactor TestFoo in cmd/foo_test.go:42-80 to use table-driven pattern. Currently has 4 sequential subtests with identical structure (setup → call → assert). Collapse into tests := []struct{ name string; input X; want Y }{...} loop.
```
</example>

<example>
Docs drift instruction:
```
Fix docs/cli-reference.md: "task list" section is missing --status flag documented in tq task list --help. Add the flag description to match CLI output.
```
</example>

## Phase 3: Exploratory review

After Phase 2 finishes, hand off to the exploratory pass — it surfaces concerns that lint, golden-rules, and docs-reviewer cannot detect.

```
Skill(skill: "gc-exploratory")
```

Run last so its findings join this session's task alongside the Phase 2 actions. If `/gc-exploratory` reports no concerns, that is a normal outcome — do nothing further.
