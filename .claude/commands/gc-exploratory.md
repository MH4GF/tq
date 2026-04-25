---
description: Exploratory review — find concerns lint/golden-rules/docs-reviewer cannot catch, then create tq actions for each
allowed-tools: Read, Glob, Grep, Bash, Agent, Skill
argument-hint: "[path]"
---

Look at the code and find things that concern you. Stay deliberately open-ended — do not narrow yourself to a fixed checklist of categories. The value of this review comes from your unguided judgment.

Scope: $ARGUMENTS (empty = whole repository).

IMPORTANT: If no concerns survive verification, report "No concerns detected" and stop. Do NOT create actions when there is nothing to fix.

## Phase 1: Exploration

Launch Explore agent(s) with a vague mission: *"Look at the code under <scope> and find anything that concerns you."* Decide yourself how many agents to run and how to slice the scope — match the breadth of the target.

In the agent prompt, state only:

- The scope (path or "whole repository")
- The mission (above)
- **Exclude** anything already caught by:
  - `golangci-lint run` (linters configured in `.golangci.yml`)
  - `go test ./internal/goldenrules/` (rules in `docs/golden-rules.md`)
  - `docs-reviewer` agent (documentation drift)
- Output format per finding: `file:line / what concerns you / likely impact`

Do NOT seed the agent with categories like "data integrity / concurrency / UX". Categories bias exploration and let unlisted concerns slip through.

## Phase 2: Verify

Read each candidate finding against the actual code. **False-positive removal is what makes this skill trustworthy.**

- Read the cited file/lines
- Trace callers and callees with Grep
- Check whether existing tests already lock in the behavior
- If you cannot reproduce the concern from the code, drop it and record why

Carry forward only findings you can defend by pointing at the source.

## Phase 3: Prioritize

Rank surviving findings P1..PN by `frequency × severity to data integrity / UX`. Higher rank for concerns that silently corrupt state or report success while nothing changed; lower for ergonomics and inefficiencies.

## Phase 4: Create child actions

For each surviving finding, run `/tq:create-action` against the current session's task.

<constraints>
- One action per finding (group only when a single PR would obviously cover both)
- Instruction MUST embed `file:line` and the verified concern — the worktree report at `.claude/plans/*.md` may be wiped, so each action must be self-sufficient
- Set `--meta '{"claude_args":["--permission-mode","auto","--worktree","<scope-name>"]}'` so workers can run unattended in parallel; derive `<scope-name>` from the affected file or feature
- Use the same task as this session (check session context)
</constraints>

Instruction shape (the contract):

```
P{N} finding from /gc-exploratory.
<file>:<line> — <verified concern>.
Impact: <user-visible consequence>.
Required: <fix direction>. Add regression test in <test file>.
```

After creating actions, report the count and stop.
