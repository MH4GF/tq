---
description: Watch GitHub notifications, classify them, and create tq actions
allowed-tools: Bash(**/scripts/gh-fetch-notifications), Bash(**/scripts/gh-mark-notification-read *), Bash(gh pr view *), Bash(gh issue view *), Bash(gh release view *), Bash(gh auth status *), Bash(tq *), Skill(tq:done)
---

GitHub notifications watcher. Fetch, classify, and create tq actions for each notification.

## Auto-mode boundaries

This skill runs under `--permission-mode auto`. State boundaries here so the classifier blocks writes.

**GitHub: read-only.** Allowed: `gh pr view`, `gh issue view`, `gh release view`, `gh auth status`, `gh-fetch-notifications`, `gh-mark-notification-read`, and `gh api` for GET requests on any path. Forbidden: any `gh` write subcommand (e.g. `gh pr create`, `gh pr merge`, `gh pr review`, `gh issue create`, `gh issue comment`) and any `gh api` call that includes `-X POST/PUT/PATCH/DELETE`, `--method`, `-f`, or `-F`.

**Repo: no mutations.** No `git push`, `git commit`, `git branch`, `git checkout`, `git merge`. No `Edit` / `Write` / `NotebookEdit`. Reads (`Read`, `Grep`, `Glob`) are fine.

**Local writes only.** Allowed writes are limited to the local tq DB (`tq action create`, `tq task create`) and marking notifications read via `gh-mark-notification-read`.

If a step requires a write not on this list, stop and report instead of attempting it.

## Steps

### 1. Fetch notifications

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-fetch-notifications
```

If the command fails, is blocked by permissions, or returns a non-zero exit code, report the error clearly and stop. Only report "No notifications" when the command succeeds with empty output.

If 0 notifications, output "No notifications" and finish.

### 2. Process each notification

For each notification, execute in order:

#### 2a. Get details

Extract repo name and number from subject_url, then fetch by subject_type:

- **PullRequest**: `gh pr view <number> --repo <owner/repo> --json url,state,author,headRefName,reviewDecision,mergeStateStatus,statusCheckRollup,isDraft,reviews,reviewRequests,comments,body`
- **Issue**: `gh issue view <number> --repo <owner/repo> --json url,state,author,body`
- **Release**: `gh release view <tag> --repo <owner/repo>` (read tag from notification title)
- **Discussion**: `gh api /repos/<owner/repo>/discussions/<number>`

If the body references other PR/Issue numbers (e.g. `#1429`, follow-up links), resolve each reference safely:
1. try `gh pr view <ref> --repo <owner/repo> --json url,state`
2. if not found, try `gh issue view <ref> --repo <owner/repo> --json url,state`
3. if both fail, record it as unresolved and continue

This feeds into the "Notification summary" section of the Co-review template (Step 2d, row 7).

#### 2b. Skip conditions

Mark as read and skip if:
- `reason=review_requested` and already reviewed (reviews contain own APPROVED/CHANGES_REQUESTED)
- `reason=review_requested` but own login (`gh auth status --active --json hosts --jq '.hosts."github.com"[0].login'`) is NOT in reviewRequests (neither as user nor as member of a requested team) — team review request where someone else was randomly assigned
- PR `state` is `MERGED` or `CLOSED` — already terminated, no merge/CI/review action needed

#### 2c. Remote action PR detection

If headRefName matches `tq-<number>-`:
```bash
# e.g. tq-42-fix-bug → action_id=42
# Build result from PR details fetched in 2a
tq action done <action_id> "Remote action created PR: <pr_url>
State: <state>, Review: <reviewDecision>, CI: <pass/fail/pending>, Draft: <yes/no>"
```
Mark as read and skip.

#### 2d. Instruction selection

For actionable notifications, select the **first matching** instruction by priority:

| Priority | Condition | Instruction |
|---|---|---|
| 1 | `reason=review_requested` + not yet reviewed | `/gh-ops:brief-pr <PR_URL>` |
| 2 | `mergeStateStatus: "BEHIND"` or conflicting | `/gh-ops:fix-conflict <PR_URL>` |
| 3 | statusCheckRollup has failure | `/gh-ops:fix-ci <PR_URL>` |
| 4 | `reviewDecision: "CHANGES_REQUESTED"` / unaddressed review comments | `/gh-ops:respond-review <PR_URL>` |
| 5 | `reviewDecision: "APPROVED"` + CI pass + mergeable | `/gh-ops:merge-pr <PR_URL>` |
| 6 | Own PR, not yet reviewed | `/gh-ops:self-review <PR_URL>` |
| 7 | Notification is informational only (Discussion, Release, mention without action signal, body referencing follow-up PR/Issue) | **Co-review template** (see below) |

If row 7 also doesn't fit and the situation truly needs free-text describing, write a detailed instruction including the PR/issue URL, context from the notification, and specific next steps. Do NOT use a slash command in that case.

##### Co-review template (row 7)

Co-review actions are discussion-oriented: the dispatched agent must surface context to the user and confirm the next step via AskUserQuestion before acting. Pre-fill the template using data collected in Step 2a.

```text
This action is a co-review discussion with the user. Before taking any action, organize the context below, then use AskUserQuestion to confirm the next step with the user.

**Notification summary**
- Type: <PullRequest / Issue / Discussion / Release>
- URL: <URL>
- State: <PR/Issue state, release tag, or discussion category>
- What happened: <follow-up announcement / mention context / branched into another PR / discussion topic / etc.>
- Related: <state of follow-up #N, related resources>

**Suggested next-step options**
- (a) Mark this task done (information acknowledged, no further tracking needed)
- (b) Register follow-up <#N> as a new task to keep tracking
- (c) <other context-specific proposal>

Present these options via AskUserQuestion and execute the user's choice. If the user picks (a), run `/tq:done`.
```

**Forbidden phrasing** in the Co-review template and any free-text instruction (these mislead the dispatched agent into short-circuiting):
- "No action required"
- "close this task" (imperative directive to the agent — the user-facing option uses "Mark this task done" instead)
- "review ... if interested" (treats reading as optional when the user actually wants to discuss)

**Excluded prompts** (never select these): `classify-gh-notification`, `classify-next-action`, `watch-gh-ops`

#### 2e. Match existing task

Identify the target project from the notification's repo (check `tq project list` metadata for matching `repos`). Then search within that project:

```bash
tq search "<keyword>" --project <project_id>
```

If no project's `repos` matches the notification, fall back to an unscoped search:

```bash
tq search "<keyword>"
```

Extract keywords from the notification title (PR number `#123`, ticket ID `PROJ-456`, repo name, feature name, etc.).

Try in order, use the first match:

1. **URL match**: notification URL exactly matches existing task URL in search results
2. **Keyword match**: search results contain a task whose title matches the notification context
3. **No match**: create new task (classify under **works** project if project is unknown)

#### 2f. Create action

```bash
# If new task needed
tq task create "<title>" --project <project_id> --meta '{"url":"<url>"}'

# Create action (instruction is the slash command from 2d)
tq action create <instruction> --task <task_id> --title "<title>"
```

### 3. Mark notifications as read

Issue one Bash tool call per notification. Do NOT batch with a `for` loop or any other shell construct — Claude Code's permission matcher dispatches on the first token, so `for id in ...; do .../gh-mark-notification-read "$id"; done` is classified as `for` and never matches `Bash(**/scripts/gh-mark-notification-read *)`. In `--permission-mode auto` (which this skill runs under) the call is denied and the action aborts. Embed each `<thread_id>` as a literal value; do not use shell variables like `$id`.

```bash
${CLAUDE_PLUGIN_ROOT}/scripts/gh-mark-notification-read <thread_id>
```

### 4. Output summary

```text
GitHub notifications processed. Fetched: N, Skipped: M, Actions created: K.
[list of each action summary]
```

### 5. Report completion

Execute `/tq:done`.

## Rules

1. One action per notification. Do not batch.
2. Use only `gh` CLI (GitHub API tokens are managed by `gh`).
3. **Never use `gh api /repos`** for PR or issue data — use `gh pr view --json` / `gh issue view --json` instead. When no dedicated subcommand exists (e.g. Discussion), fall back to `gh api` with a GET request, subject to the Auto-mode boundaries above.
