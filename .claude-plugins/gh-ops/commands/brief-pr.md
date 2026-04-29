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

Write the digest to `.claude/tmp/brief-pr-<pr_number>.md`. **Optimize for scannability over completeness** — the reviewer must grasp the whole PR within the first screen, then drill in.

### 3.1 Architecture sketch (mandatory)

From the diff (Phase 1) and any files already opened while reading it, draw an ASCII box-and-arrow diagram showing only the parts this PR touches (or directly interacts with) — not the whole system. Keep it ≤12 lines.

```
[Frontend] ──► [API GW] ──► [Lambda] ──► [DynamoDB]
                                │
                                ▼
                            [SNS Topic]
```

If the PR is purely textual (docs / config only) and a flow diagram does not apply, substitute a short "what changed where" block in the same code fence.

### 3.2 Stack / dependency tree (conditional)

From the Phase 0c JSON (`body`, `commits[].messageHeadline`, `baseRefName`), look for any of: `stack`, `Block A/B`, `closes #<n>`, `related to #<n>`, a base branch other than `main`/`master`. If at least one signal fires, add a small ASCII tree showing parent/child PRs or issues. Otherwise omit the section entirely (no header, no placeholder).

```
#<base-pr> (base) ──► #<this-pr> (this PR) ──► #<followup-pr> (followup)
```

### 3.3 Skeleton

Fill this exact structure. Sections without an `<!-- omit ... -->` comment are mandatory; sections with one may be dropped per the stated condition.

````markdown
# PR #<pr_number>: <title>
Author: <author> · +<additions> / -<deletions> · <changedFiles> files

## Architecture

```
<ASCII diagram from 3.1>
```

## Stack

```
<ASCII tree from 3.2>
```
<!-- omit entire Stack section if 3.2 detected no signal -->

## TL;DR
- **What**: <one line — the core change>
- **Scope**: <one line — surface area / how far it reaches>
- **Risk**: <one line — what could break / who is affected>

## Reading order

| Priority | Area | Path | Look for |
|---|---|---|---|
| ★3 | <logical unit> | `path:line` | <one line> |
| ★2 | ... | ... | ... |
| ★1 | ... | ... | ... |

## Impact

```
[Added]    <new files / new behaviors>
[Changed]  <modified call sites, layers>
[New deps] <new external libs / APIs / services>
[Breaking] <breaking changes / migrations>
```
<!-- omit any line whose value would be empty; omit the whole block only if all four are empty -->

## Decision points

<!-- One card per judgment the human must make. Prepend ⚠ to the title only when the reviewer should pause; omit it for routine calls. -->

```
[⚠] <title of the decision>
Decision  <what was chosen>
Why       <rationale>
Risk      <tradeoff / rejected alternative>
Verify    <what the reviewer must check>
```
<!-- omit entire Decision points section if no judgment calls exist -->

## AI reviewer triage

🎯 Take seriously (<N>):
- **`path:line`** *(<bot-login>)* — <one-sentence summary>
- ...

🔧 Author likely handled: <N> · 🔇 Noise: <N>
<!-- omit entire AI reviewer triage section if Phase 2 found zero bot comments -->

## Other reviewer comments

- **<author>** on `path:line` — <one-line gist>
- ...
<!-- omit entire section if Phase 2 found zero human comments -->

## New commits since your last review

- `<oid>` <messageHeadline>
- ...
<!-- omit entire section if `<new_commits>` from Phase 0c is empty -->
````

### 3.4 Formatting discipline

Hard rules:

- **TL;DR is exactly three bullets.** If a fourth point seems essential, it belongs in Decision points or Impact, not TL;DR.
- **Tables only when comparison is the point.** Reading order earns its table because priority + path + intent are columns. Do not table-format simple lists; use bullets or the ASCII Impact block instead.
- **One line per AI triage entry.** No nested sub-bullets, no rationale paragraphs — the path lets the reviewer open the file if they want detail.
- **At most four Decision cards.** If you have more, you are over-listing — keep the highest-stakes ones and let the reviewer ask follow-ups in Phase 4.
- **Reading order rows: ★3 = blocking review, ★2 = warrants attention, ★1 = skim only.** Order rows by priority, not by path.
- **Emphasis carries weight.** Bold = the keyword or conclusion. Italic = author/reviewer attribution. Block-quote `>` = verbatim quote (commit message, comment text). If everything is bold, nothing is.
- **Whitespace is intentional.** Insert a blank line between sections. The first screen must not look like a wall of text.

After writing, tell the user (in chat output, not in the file) where the digest was saved:

> Digest written to `.claude/tmp/brief-pr-<pr_number>.md`.

## Phase 4: Ask Devin (interactive)

End the turn so the user knows the skill is staying open:

> Digest ready (diff, related files, all reviewer comments are in context). Skim the architecture diagram and TL;DR first, then ask anything for follow-up. Run `/tq:done` when finished.

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
