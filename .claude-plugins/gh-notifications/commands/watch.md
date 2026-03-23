---
description: Watch GitHub notifications, classify them, and create tq actions
allowed-tools: Bash(gh *), Bash(tq *)
---

GitHub notifications watcher. Fetch, classify, and create tq actions for each notification.

## Steps

### 1. Fetch notifications

```bash
gh api /notifications --paginate --jq '.[] | {id: .id, reason: .reason, subject_type: .subject.type, title: .subject.title, repo: .repository.full_name, subject_url: .subject.url}' 2>/dev/null
```

If 0 notifications, output "通知なし" and finish.

### 2. Process each notification

For each notification, execute in order:

#### 2a. Get details

Extract repo name and number from subject_url, then fetch by subject_type:

- **PullRequest**: `gh pr view <number> --repo <owner/repo> --json url,state,author,headRefName,reviewDecision,mergeStateStatus,statusCheckRollup,isDraft,reviews`
- **Issue**: `gh issue view <number> --repo <owner/repo> --json url,state,author`
- **Discussion/Release**: fetch via `gh api`

#### 2b. Skip conditions

Mark as read and skip if:
- state is merged/closed
- Discussion/Release
- `reason=review_requested` and already reviewed (reviews contain own APPROVED/CHANGES_REQUESTED)

#### 2c. Remote action PR detection

If headRefName matches `tq-<number>-`:
```bash
# e.g. tq-42-fix-bug → action_id=42
# Build result from PR details fetched in 2a
tq action done <action_id> "Remote action created PR: <pr_url>
State: <state>, Review: <reviewDecision>, CI: <pass/fail/pending>, Draft: <yes/no>"
```
Mark as read and skip.

#### 2d. Prompt selection

For actionable notifications, select the **first matching** prompt by priority:

| Priority | Condition | Prompt |
|---|---|---|
| 1 | `reason=review_requested` + not yet reviewed | `review-pr` |
| 2 | `mergeStateStatus: "BEHIND"` or conflicting | `fix-conflict` |
| 3 | statusCheckRollup has failure | `fix-ci` |
| 4 | `reviewDecision: "CHANGES_REQUESTED"` / unaddressed review comments | `respond-review` |
| 5 | `reviewDecision: "APPROVED"` + CI pass + mergeable | `merge-pr` |
| 6 | Own PR, not yet reviewed | `self-review` |
| 7 | Other implementation/fix requests | `implement` |

**Excluded prompts** (never select these): `classify-gh-notification`, `classify-next-action`, `watch-gh-notifications`

#### 2e. Match existing task

Use `tq search "<keyword>"` to find matching tasks. Extract keywords from the notification title (PR number `#123`, ticket ID `PROJ-456`, repo name, feature name, etc.) and search.

Try in order, use the first match:

1. **URL match**: notification URL exactly matches existing task URL in search results
2. **Keyword match**: search results contain a task whose title matches the notification context
3. **No match**: create new task (classify under **works** project if project is unknown)

#### 2f. Create action

```bash
# If new task needed
tq task create --project <project_name> --title "<title>" --url "<url>"

# Create action
tq action create <prompt> --task <task_id> --title "<title>"
```

### 3. Mark notifications as read

```bash
gh api -X PATCH /notifications/threads/<thread_id>
```

### 4. Output summary

```text
GitHub通知処理完了。取得: N件、スキップ: M件、アクション作成: K件。
[list of each action summary]
```

### 5. Report completion

Execute `/tq:done`.

## Rules

1. One action per notification. Do not batch.
2. Use only `gh` CLI (GitHub API tokens are managed by `gh`).
