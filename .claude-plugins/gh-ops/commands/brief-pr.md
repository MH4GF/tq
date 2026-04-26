---
description: PR comprehension copilot — generate digest + Ask Devin chat
argument-hint: "<PR_URL>"
allowed-tools: Bash(gh pr view *), Bash(gh pr diff *), Bash(gh pr checkout *), Bash(**/scripts/gh-pr-feedback *), Bash(gh auth status *), Bash(git status --porcelain), Bash(git rev-parse *), Bash(git checkout -), Read, Grep, Glob, Write
---

# Brief PR — Comprehension Copilot

**Goal**: scale the human reviewer's understanding of the PR. Generate a local digest, then stay open for follow-up questions ("Ask Devin" mode). The reviewer keeps the verdict; this skill never posts to GitHub.

## Out of scope (do NOT do)

- Verdict suggestions (Approve / Comment / Request changes)
- Per-line GitHub posting (`gh pr review --comment`, `gh api -X POST` of any kind)
- Bug enumeration / Code quality / Design / Testing exhaustive checklists — CI bots already do this
- Style enforcement, naming debates, design philosophy arguments
- Autonomous termination by Claude — only the user ends the session via `/tq:done`

## Phase 0a: Dirty check

```bash
git status --porcelain
```

If output is non-empty → **abort** and tell the user:

> Working tree is dirty. Commit or stash before re-running.

Do not proceed to any further phase. No cleanup is needed because no checkout has happened yet.

## Phase 0b: Resolve PR identity and own login

Parse `$ARGUMENTS` (PR URL) for `<owner>`, `<repo>`, `<pr_number>`.

```bash
gh auth status --active --json hosts --jq '.hosts."github.com"[0].login'
```

Record this as `<own_login>`.

## Phase 0c: Fetch PR data and decide skip

Single fetch — reuse this JSON in Phases 1 and 3:

```bash
gh pr view "$ARGUMENTS" --json title,body,author,baseRefName,headRefName,additions,deletions,changedFiles,reviews,commits
```

From the JSON:
- Find the most recent review where `author.login == <own_login>` and `state` is one of `APPROVED`, `CHANGES_REQUESTED`, `COMMENTED`. Record its `commit.oid` as `<reviewed_sha>`.
- Take the head commit (last entry of `commits`, or the one with the latest `committedDate`) and record its `oid` as `<head_sha>`.

Decide:
- If no own review exists → proceed (first-time review).
- If `<head_sha> == <reviewed_sha>` → **skip** with:

  > Already reviewed; no new commits since `<reviewed_sha>`. Skipping digest.

  Do not proceed. No checkout has happened, no cleanup needed.

- Otherwise → proceed; record the commits whose `oid` differs from `<reviewed_sha>` and that come after it in the list as `<new_commits>` (oid + messageHeadline) for Phase 3. Force-pushes also flow through this branch because every `oid` changes.

## Phase 1: Local context expansion

### 1. Record current branch

```bash
git rev-parse --abbrev-ref HEAD
```

Record as `<original_branch>` for restoration in Phase 5.

### 2. Check out PR locally

```bash
gh pr checkout "$ARGUMENTS"
```

### 3. Fetch the diff

PR metadata was already fetched in Phase 0c — reuse that JSON. Only the diff is missing:

```bash
gh pr diff "$ARGUMENTS"
```

### 4. Load repo review-instruction files (if present)

For each path that exists at the repo root, Read it and use it to shape digest tone and emphasis:
- `CLAUDE.md`
- `REVIEW.md`
- `.cursorrules`

If none exist, skip silently.

## Phase 2: Reviewer feedback (load all, triage AI bots)

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-pr-feedback "$ARGUMENTS"
```

Returns `{reviews, inlineComments}` for the PR — both arrays unfiltered (humans + bots). Hold the full output in context for Phase 4 questions.

### 2a. Split by author type

Partition each comment into **AI bot** vs **human** by `author.login`:

- AI bot: login ends with `[bot]`, OR matches one of the known AI bot handles below
  - `coderabbitai`, `devin-ai-integration`, `claude`, `github-actions`, `aws-security-agent`
  - Extensible — treat any other `*[bot]` whose body looks like AI review output as a bot
  - **Exclusion**: never classify `<own_login>` as a bot, even if the login string happens to match one of the known handles
- Human: anything else (including `<own_login>`)

### 2b. Triage AI bot comments only

AI bot output is high-volume and low signal-to-noise. Classify each AI bot comment into exactly one bucket:

- 🎯 **Take seriously** — security, data integrity, blast-radius concerns, anything the human reviewer must judge
- 🔧 **Author likely handled** — nits, auto-fixable issues, already-resolved threads (`resolved: true`)
- 🔇 **Noise** — style preferences, duplicates, false positives

### 2c. Human comments

Do **not** triage human comments (their signal-to-noise is high; classifying them as "noise" is rude and risks dismissing real concerns). Just collect them for the "Other reviewer comments" digest section: count + per-comment summary (`path:line` if inline, author, one-line gist).

## Phase 3: Digest generation

Write the digest to `.claude/tmp/brief-pr-<pr_number>.md` with this structure:

```markdown
# PR #<pr_number>: <title>
Author: <author> · +<additions> / -<deletions> · <changedFiles> files

## TL;DR
<one paragraph: what the PR is trying to accomplish>

## Reading map (logical units, dependency order)
1. **<Group A>** (`path/to/file_a.ts:10-30`, `path/to/file_b.ts:1-20`)
   - What is happening: ...
   - Why it matters: ...
2. **<Group B>** (...)
   - ...

## Blast radius
- Surface area: <touched layers, callsite count>
- Backward compatibility: <breaking? migration needed?>
- External dependencies: <new deps / external APIs>

## Decision points (for the human reviewer)
- <e.g. data migration is irreversible / competing design choices were made / new external dependency introduced>
- For each: why a human call is required, and any rejected alternatives.

## AI reviewer triage
- 🎯 Take seriously (<N>):
  - `path:line` — <summary>
  - ...
- 🔧 Author likely handled: <N> (details omitted)
- 🔇 Noise: <N> (details omitted)

## Other reviewer comments
- <author> on `path:line` — <one-line gist>
- ...
```

**Grouping rule for Reading map**: group hunks by logical unit (feature, layer, data flow), not by alphabetical file order. Order groups by dependency direction so the reader can follow top-down.

**If new commits arrived after the user's prior review** (`<new_commits>` from Phase 0c is non-empty), append a final section:

```markdown
## New commits since your last review
- `<oid>` <messageHeadline>
- ...
```

Section omission rules:
- AI reviewer triage → omit if Phase 2 found zero bot comments.
- Other reviewer comments → omit if Phase 2 found zero human comments.
- New commits since your last review → omit if `<new_commits>` is empty.

After writing, tell the user (in chat output, not in the file):

> Digest written to `.claude/tmp/brief-pr-<pr_number>.md`. Preview with:
>
> ```
> cc-human-review .claude/tmp/brief-pr-<pr_number>.md
> ```

## Phase 4: Ask Devin (interactive)

End the turn with this exact line (or close paraphrase) so the user knows the skill is staying open:

> PR context loaded (diff, related files, all reviewer comments). Ask anything for follow-up. Run `/tq:done` when finished.

For every subsequent user turn:
- Answer using Read / Grep against the locally checked-out PR branch (Phase 1 already did the checkout — files at HEAD are the PR head).
- Cite specific file paths and line numbers as evidence.
- Do not propose code changes, do not post anything to GitHub, do not declare the conversation finished.

**Never auto-terminate.** Only `/tq:done` ends the session.

## Phase 5: Cleanup (only when `/tq:done` runs)

When the user invokes `/tq:done` (or this skill is otherwise about to end), restore the original branch:

```bash
git checkout -
```

This returns to `<original_branch>` recorded in Phase 1.

Leave `.claude/tmp/brief-pr-<pr_number>.md` in place — it is a reusable artifact.
