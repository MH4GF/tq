---
description: Self-review a PR before requesting review
argument-hint: "<PR_URL>"
---

# Self-Review

**Goal: Make the PR worth a reviewer's time.** Complete quality checks and prerequisite verification, then mark ready if everything passes.

Not an exhaustive code review. Focus on "obvious issues you can catch before submitting" and "prerequisites for marking ready."

## Phase 1: Analysis (no changes)

Collect findings only. Do not make fixes or post comments.

### 1. CI status check

```bash
gh pr checks $ARGUMENTS
```

If failures/errors exist → check the job logs and identify the cause.

**IMPORTANT: On CI failure, skip the remaining steps (3–8) and start fixing CI immediately.** Reviewing diffs or descriptions while CI is broken is wasted effort since CI fixes will invalidate them. After fixing and pushing, restart the self-review from the beginning.

### 2. Conflict check

```bash
gh pr view $ARGUMENTS --json mergeable,mergeStateStatus
```

mergeStateStatus is "DIRTY" and mergeable is "CONFLICTING" → conflicts exist.

**IMPORTANT: On conflict detection, skip the remaining steps (3–8) and start resolving conflicts immediately.** Reviewing diffs while conflicts exist is wasted effort. After resolving and pushing, restart the self-review from the beginning.

### 3. Diff noise detection

```bash
gh pr diff $ARGUMENTS
```

Look for:
- Debug code (console.log, print, commented-out code)
- Unrelated changes (formatter auto-fixes, lock file diffs)
- Abandoned TODO/FIXME

If the diff is large (roughly 400+ lines) → note as a split candidate.

### 4. PR description vs implementation consistency

```bash
gh pr view $ARGUMENTS --json body,title
```

Check:
- Title accurately represents the change
- Motivation (WHY) is clear
- Description does not contradict the implementation
- No stale descriptions remain

### 5. Commit granularity

```bash
gh pr view $ARGUMENTS --json commits
```

- No single commit mixes multiple concerns
- Commit messages describe what changed

### 6. Reviewability

Read the diff from a reviewer's perspective:
- Is the intent of each change easy to follow?
- Do tests correspond to the changes?
- Any obvious gaps in error handling?
- Are breaking changes (API changes, DB migrations, new env vars) clearly called out?

### 7. Anticipate reviewer questions

Identify places where a reviewer would ask "why?" — the ideal outcome is a zero-comment Approve.

Look for:
- Non-obvious design decisions (rationale for chosen approach, rejected alternatives)
- Unclear naming (variables, functions, types)
- Implicit side effects (state mutations, external calls, performance impact)
- Deviations from existing patterns

Record resolution for each:
- **Code fix** (preferred): rename, refactor, or add comments to eliminate the question
- **PR comment**: add inline diff comments for background/trade-offs that code cannot express

### 8. Unresolved comments check

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-unresolved-threads $ARGUMENTS
```

If unresolved comments exist → record as a finding. Handle via `/gh-notifications:respond-review $ARGUMENTS`.

## Phase 2: Report and discuss

MUST: Present all findings to the user and discuss the resolution for each. Do not start fixing without agreement.

Report findings in this format:

| # | Category | Detail | Proposed action |
|---|----------|--------|-----------------|
| 1 | CI failure | Log summary | Fix approach |
| 2 | Conflict | N files conflicting | Resolution approach |
| 3 | Noise | Location | Remove or justify |
| 4 | Unresolved comments | N threads | Handle via respond-review |
| ... | ... | ... | ... |

For items requiring judgment (value selection, naming, design decisions), provide precedent research and rationale.

If 0 findings → report that and finish.

## Phase 3: Execute

Implement fixes per the agreed plan. For unresolved comments, follow `/gh-notifications:respond-review $ARGUMENTS`.

## Phase 4: Mark ready

After completing Phases 1–3, decide whether the PR can be marked ready for review.

**IMPORTANT: If any concern remains, do NOT propose marking ready.** Report the concern and wait for user instructions.

### 1. Prerequisite check

Determine prerequisites for marking this PR ready based on task info, diff, and PR description. Confirm with the user.

Check:
- **Environment setup**: If infra/config changes are needed, are they complete?
- **Manual verification**: If manual QA/E2E testing is required, has it been done?
- **Dependent PRs**: If other PRs must merge first, have they merged?

If prerequisites exist → confirm status with the user. Block ready if any are incomplete.

### 2. Ready decision

Propose marking ready ONLY if ALL of the following are true:
- All Phase 1–3 findings are resolved
- All prerequisites are satisfied
- Zero items where judgment is uncertain

Even when proposing ready, wait for user approval. Never auto-execute.

```bash
gh pr ready $ARGUMENTS
```
