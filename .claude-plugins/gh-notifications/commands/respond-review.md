---
description: Respond to review comments on a PR
argument-hint: "<PR_URL>"
allowed-tools: Bash(*/scripts/gh-unresolved-threads *), Read, Edit, Write, Grep, Glob
---

# Respond to Review Comments

Respond to reviewer comments. Process in batches by phase, not one-by-one sequentially.

## Phase 1: Analysis (no changes)

### 1. Fetch unresolved comments

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-unresolved-threads $ARGUMENTS
```

### 2. Classify

Classify each comment:

**No action needed (do NOT resolve):**
- Code explanation comments, FYI information sharing, already-addressed findings
- Leave the thread unresolved. Reviewer insights and supplementary info are worth keeping on the thread

**No code change needed but resolve required:**
- Will address in a separate issue/PR → post the issue URL and resolve
- Not addressing a nit/optional → state the reason and resolve

**Action required:**
- Threads with open discussion or awaiting a response

## Phase 2: Report and discuss

MUST: Present the classification to the user and discuss the resolution for each comment. Do not start fixing without agreement.

| # | Thread | Classification | Proposed action |
|---|--------|----------------|-----------------|
| 1 | Summary of the location | Action required / No action | Fix approach or reason for no action |
| ... | ... | ... | ... |

For items requiring judgment (value selection, naming, design decisions), provide precedent research and rationale.

If 0 unresolved comments → report that and finish.

## Phase 3: Execute

Implement per the agreed plan, in batch.

### Step 1: Declare intent

Comment "Will address" on all threads (communicate intent before starting work).

### Step 2: Implement

Implement all fixes together and commit.

### Step 3: Push

Push commits to remote. Required for GitHub to linkify commit hashes.

### Step 4: Report completion

Comment and resolve all threads in batch.

Completion comments must include the commit hash and rationale. Keep it brief when self-evident.

<example>
# Good: commit hash + rationale
Addressed in abc1234. The nil check was missing, which could cause a panic.

# Good: self-evident case
Fixed in abc1234. Typo fix.

# Bad: no commit hash
Addressed.
</example>

### Step 5: Re-request review

After all responses are complete, re-request review from the reviewer.

```bash
gh pr edit $ARGUMENTS --add-reviewer <REVIEWER_LOGIN>
```

## GraphQL Reference

### Reply to a thread

Use the `id` (THREAD_ID) from `gh-unresolved-threads` output to reply:

```bash
gh api graphql -f query='
  mutation {
    addPullRequestReviewThreadReply(input: {
      pullRequestReviewThreadId: "<THREAD_ID>"
      body: "Reply content"
    }) { comment { id } }
  }'
```

### Resolve a thread

```bash
gh api graphql -f query='
  mutation {
    resolveReviewThread(input: {threadId: "<THREAD_ID>"}) {
      thread { isResolved }
    }
  }'
```

When both replying and resolving, execute reply first, then resolve.
