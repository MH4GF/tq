---
description: Judge and execute PR merge
argument-hint: "<PR_URL>"
allowed-tools: Bash(gh pr view *), Skill(gh-ops:self-review)
---

# Merge Decision

Decision level: **L1 (report + propose)** — default. AI presents rationale for all criteria and waits for user confirmation.

## Steps

### 1. Self-review

Run `/gh-ops:self-review $ARGUMENTS` and confirm no unresolved findings remain. Do not proceed to merge if findings remain.

**Non-blocking threads**: Threads that are purely informational/knowledge-sharing (FYI, no action needed) do not block merge even if unresolved. All other unresolved threads must be resolved (see `/gh-ops:respond-review $ARGUMENTS`). When in doubt, treat as a blocker and confirm with the user.

### 2. Merge gate

Approval is a necessary condition, not a sufficient one. Verify all of the following:

```bash
gh pr view $ARGUMENTS --json state,title,mergeable,reviewDecision,reviews,mergeStateStatus,updatedAt
```

#### Approval status (required)

Check the project's CLAUDE.md for the required number of approvals. Insufficient approvals → cannot merge.

#### Divergence from main

Use the already-fetched `mergeable`, `mergeStateStatus`, `updatedAt`:
- mergeable is not MERGEABLE → conflict resolution needed
- Last updated more than 3 days ago → check whether main changes cause semantic conflicts
- mergeStateStatus is BLOCKED → CI failure or branch protection rule violation

#### CI / Dependencies

- All CI checks green? (If CI is running → evaluate other criteria first, final decision after CI completes)
- Any dependent PRs? (merge order constraints)

### 3. Execute merge

After user approval, merge:

```bash
gh pr merge $ARGUMENTS --merge
```

After execution, report any follow-up PRs if they exist.

## Decision Log

Record misjudgments here and update criteria accordingly.

<!-- Example:
- YYYY-MM-DD: Proposed merging PR #NNN but overlooked XXX → added "XXX" to criterion N
-->
