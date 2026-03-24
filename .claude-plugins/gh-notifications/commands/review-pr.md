---
description: Review another person's PR
argument-hint: "<PR_URL>"
allowed-tools: Bash(gh pr view *), Bash(gh pr diff *), Bash(gh pr checks *), Read, Grep, Glob
---

# PR Review

Review another person's PR, providing feedback on code quality, design, and testing.

## Phase 1: Analysis (no changes)

### 1. Understand the PR

```bash
gh pr view $ARGUMENTS --json title,body,author,baseRefName,headRefName,additions,deletions,changedFiles
```

### 2. Get the diff

```bash
gh pr diff $ARGUMENTS
```

### 3. Check CI status

```bash
gh pr checks $ARGUMENTS
```

### 4. Review criteria

Examine the diff against these criteria:

**Code quality:**
- Potential bugs (nil references, boundary values, missing error handling)
- Security concerns (input validation, authentication/authorization)
- Performance issues (N+1 queries, unnecessary allocations)

**Design:**
- Consistency with existing architecture
- Appropriate separation of concerns
- Naming accurately conveys intent

**Testing:**
- Tests correspond to the changes
- Edge cases are covered

**Other:**
- Breaking changes
- Documentation updates needed

## Phase 2: Report and discuss

MUST: Present all findings to the user and discuss the review verdict before posting. Do not post the review without agreement.

| # | Severity | Location | Finding | Suggestion |
|---|----------|----------|---------|------------|
| 1 | high/medium/low | file:line | Issue description | Proposed fix |
| ... | ... | ... | ... | ... |

Propose the review verdict:
- **Approve** — no issues or only minor nits
- **Comment** — suggestions but not blocking
- **Request changes** — issues that must be fixed before merge

## Phase 3: Post review

After user approval, post the review on GitHub:

```bash
gh pr review $ARGUMENTS --approve --body "<review content>"
gh pr review $ARGUMENTS --comment --body "<review content>"
gh pr review $ARGUMENTS --request-changes --body "<review content>"
```
