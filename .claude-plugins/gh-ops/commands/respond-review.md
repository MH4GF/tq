---
description: Respond to review comments on a PR
argument-hint: "<PR_URL>"
allowed-tools: Bash(**/scripts/gh-prepare-review-replies *), Bash(**/scripts/gh-reply-review-thread *), Bash(**/scripts/gh-resolve-review-thread *), Bash(gh pr edit *), Read, Edit, Grep, Glob
---

# Respond to Review Comments

Respond to reviewer comments. Process in batches by phase, not one-by-one sequentially.

## Phase 1: Generate draft file

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-prepare-review-replies $ARGUMENTS
```

The script writes a Markdown draft to `.claude/tmp/pr${NUMBER}-review-replies-${TIMESTAMP}.md` containing every unresolved thread (Thread ID, file:line, author, every quoted comment in the thread joined by `---`, empty Classification/Reply draft/Edit plan fields). It prints the generated path on stdout.

If stdout is empty (stderr says `No unresolved threads`) → report and finish.

## Phase 2: Classify and draft in tmp file

Read the file path printed by Phase 1 and fill in each thread.

### Editing rules (the draft is the user's source of truth)

- Edit ONLY the `Classification` line, the `Reply draft` block, and the `Edit plan` block. Leave every other byte — heading, Thread ID, quoted comment — exactly as the script wrote it.
- NEVER modify, summarize, paraphrase, reorder, or delete the quoted comment lines (lines starting with `>`). The user reviews the draft against that quote; rewriting it forces them to cross-check GitHub for every reply and breaks the workflow.
- NEVER reference threads by position or by any other ad-hoc label inside `Reply draft` (e.g. `same as the first thread`, `see thread above`). The GitHub review UI shows no such ordering, so reviewers cannot resolve the reference. If you need to point at another thread, cite its `file:line` directly.

### Fields to fill in

- **Classification**: one of
  - `No action` — code explanation, FYI, already-addressed findings. Do NOT resolve.
  - `Resolve without code change` — nit declined / deferred to another issue. Resolve after replying.
  - `Action required` — code change needed. Resolve after the fix.
- **Reply draft**: the body to post on the thread.
  - `Action required`: write the body assuming an `addressed in abc1234` style commit hash will be appended after the fix.
  - `Resolve without code change`: write the final body to be posted as-is.
  - `No action`: usually empty. Fill in only if you want to post a comment without resolving.
- **Edit plan**: how the code will be changed. Fill this in **only** when `Classification` is `Action required` — leave the `_(n/a)_` placeholder for the other two. The plan is what the user signs off on before Phase 3 Step 2 runs, so it must be specific enough that the user can redirect the approach without reading the diff.
  - Include: target file(s) / function(s) being touched, the intended change (with alternatives when there's a real choice to make), side effects or out-of-scope items being deliberately left alone, and how the change will be verified (test added / existing test exercised / manual check).
  - Keep it proportional to the comment. A typo fix needs a one-liner ("fix typo in `foo.go:42`"). A behavioural change or design call needs the alternatives and the rationale for the chosen one.
  - If the right fix is genuinely unclear, write the plan as a question / option list and surface it to the user before Phase 3 instead of guessing.

For judgment calls (naming, value selection, design decisions), include precedent research and rationale in the draft AND the edit plan.

### Example: what to edit vs what to preserve

Before (script output):

```markdown
## src/foo.ts:42 by @coderabbitai[bot]

- **Thread ID**: `PRRT_xxx`

> Consider extracting this into a helper — it's duplicated in `bar.ts:88`.

**Classification**: _(No action / Resolve without code change / Action required)_

**Reply draft**:

_(empty)_

**Edit plan** _(Action required only — leave as `_(n/a)_` otherwise)_:

_(n/a)_
```

After (your edit — heading / Thread ID / quote untouched):

```markdown
## src/foo.ts:42 by @coderabbitai[bot]

- **Thread ID**: `PRRT_xxx`

> Consider extracting this into a helper — it's duplicated in `bar.ts:88`.

**Classification**: Action required

**Reply draft**:

Extracted into `extractFoo()` in `src/util/foo.ts`. Both call sites updated.

**Edit plan** _(Action required only — leave as `_(n/a)_` otherwise)_:

- Target: `src/foo.ts:42` and `src/bar.ts:88` (the two duplicated blocks).
- Change: extract the shared body into `extractFoo(input)` in a new `src/util/foo.ts`; replace both sites with calls to it. Keep the signature `(input: FooInput) => FooResult` so neither caller needs adapting.
- Alternative considered: inline at one site and delete the other. Rejected — `bar.ts` has a slightly different surrounding context, deleting it would change behaviour.
- Out of scope: the third near-duplicate at `baz.ts:120` looks similar but takes a different input shape; leave it for a follow-up.
- Verification: existing `foo.test.ts` covers both call sites; add one direct unit test for `extractFoo` to lock the contract.
```

For `No action` / `Resolve without code change` threads, leave `Edit plan` as the `_(n/a)_` placeholder — there is no code change to plan.

After editing, present the file to the user and reach agreement on both the Reply draft AND the Edit plan. The user may edit the file directly. Do NOT start Phase 3 without agreement — the Edit plan is what Phase 3 Step 2 implements, so an unreviewed plan means an unreviewed code change.

**Skip Phase 3 entirely** when every thread is `No action` and every Reply draft is empty. Report `all no-op` and finish.

## Phase 3: Execute

Execute strictly in this order. Each step is a no-op if its target set is empty.

### Step 1: Post "will address" replies
Post `Will address` on every `Action required` thread (intent declaration before starting work). Run all posts **in parallel** (single message, multiple Bash tool calls).

Skip if no `Action required` threads.

### Step 2: Apply code changes
Implement every fix per the agreed `Edit plan` for each `Action required` thread. The Edit plan is the source of truth for what to change; the Reply draft only describes what happened. Bundle all changes into one commit.

If, while implementing, you discover the Edit plan is wrong or incomplete, stop and re-sync with the user before continuing — do not silently deviate from the plan that was agreed in Phase 2.

Skip if no `Action required` threads.

### Step 3: Push
`git push` to remote. Required so GitHub linkifies the commit hash.

Skip if Step 2 produced no commit.

### Step 4: Post replies
- `Action required`: post the completion comment with the commit hash spliced into the Reply draft.
- `Resolve without code change`: post the Reply draft as-is.
- `No action` with a Reply draft: post the Reply draft as-is.

Run all posts **in parallel** (single message, multiple Bash tool calls).

**Do NOT resolve in this step.** Posting must always precede resolving.

<example>
# Good: commit hash + rationale
Addressed in abc1234. The nil check was missing, which could cause a panic.

# Good: self-evident case
Fixed in abc1234. Typo fix.

# Bad: no commit hash on an Action required thread
Addressed.
</example>

### Step 5: Resolve threads
Resolve every thread that received a reply in Step 4 EXCEPT `No action` threads (those keep the comment without resolving). Run all resolves **in parallel**.

### Step 6: Re-request review
Re-request review from human reviewers. Skip bot reviewers (e.g., `devin-ai-integration[bot]`).

```bash
gh pr edit $ARGUMENTS --add-reviewer <REVIEWER_LOGIN>
```

## GraphQL Reference

### Reply to a thread

Use the Thread ID listed in the tmp file generated by `gh-prepare-review-replies`:

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-reply-review-thread "<THREAD_ID>" "Reply content"
```

### Resolve a thread

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-resolve-review-thread "<THREAD_ID>"
```

When both replying and resolving, execute reply first, then resolve.
