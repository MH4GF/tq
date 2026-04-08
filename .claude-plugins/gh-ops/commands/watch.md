---
description: Watch GitHub notifications, classify them, and create tq actions
allowed-tools: Bash(**/scripts/gh-fetch-notifications), Bash(**/scripts/gh-mark-notification-read *), Bash(gh pr view *), Bash(gh issue view *), Bash(tq *), Skill(tq:done)
---

GitHub notifications watcher. Fetch, classify, and create tq actions for each notification.

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

- **PullRequest**: `gh pr view <number> --repo <owner/repo> --json url,state,author,headRefName,reviewDecision,mergeStateStatus,statusCheckRollup,isDraft,reviews,reviewRequests`
- **Issue**: `gh issue view <number> --repo <owner/repo> --json url,state,author`
- **Discussion/Release**: fetch via `gh api`

#### 2b. Skip conditions

Mark as read and skip if:
- `reason=review_requested` and already reviewed (reviews contain own APPROVED/CHANGES_REQUESTED)
- `reason=review_requested` but own login (`gh api /user --jq .login`) is NOT in reviewRequests (neither as user nor as member of a requested team) — team review request where someone else was randomly assigned

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
| 1 | `reason=review_requested` + not yet reviewed | `/gh-ops:review-pr <PR_URL>` |
| 2 | `mergeStateStatus: "BEHIND"` or conflicting | `/gh-ops:fix-conflict <PR_URL>` |
| 3 | statusCheckRollup has failure | `/gh-ops:fix-ci <PR_URL>` |
| 4 | `reviewDecision: "CHANGES_REQUESTED"` / unaddressed review comments | `/gh-ops:respond-review <PR_URL>` |
| 5 | `reviewDecision: "APPROVED"` + CI pass + mergeable | `/gh-ops:merge-pr <PR_URL>` |
| 6 | Own PR, not yet reviewed | `/gh-ops:self-review <PR_URL>` |

If no condition matches, do NOT use a slash command. Instead, write a detailed free-text instruction describing what needs to be done — include the PR/issue URL, the context from the notification, and specific next steps.

**Excluded prompts** (never select these): `classify-gh-notification`, `classify-next-action`, `watch-gh-ops`

#### 2e. Match existing task

Use `tq search "<keyword>"` to find matching tasks. Extract keywords from the notification title (PR number `#123`, ticket ID `PROJ-456`, repo name, feature name, etc.) and search.

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
