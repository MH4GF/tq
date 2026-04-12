---
description: Inventory and organize open tasks - review status, propose cleanup, execute
argument-hint: "[project_name]"
---

# tq triage

Inventory and organize open tasks.

## Steps

### 1. Collect

Run `tq task list --status open` to fetch open tasks with their full action history. Filter by `--project` if `$ARGUMENTS` is given.

### 2. Project consistency check

Detect and fix tasks that landed in the wrong project due to auto-creation (e.g. `gh-ops:watch`). Run `tq project list` to get the project list with metadata, then infer the expected project from each task's `metadata.url`, title, and action history (instruction / result), and compare against the current `project_id`.

**Present mismatches and fix**: If mismatches are found, present them in the table below and confirm **one at a time** via AskUserQuestion (choices: move / skip). On each approval, run `tq task update <ID> --project <new_id>`. Do not batch-approve even if many tasks are affected (to minimize the impact of misjudgment).

| ID | Title | Current | Expected | Evidence |
|---|---|---|---|---|
| 420 | Respond to PR #55 | works | immedio | metadata.url: github.com/immedioinc/app/pull/55 |

### 3. Phase detection

Read each task's action history in chronological order and determine its current phase.

**Identifying real-work actions**: `classify-next-action` is an auto-routing action and does not represent task progress. Treat all actions other than `classify-next-action` as real-work actions and read the most recent result.

**Phase criteria**:

- **Not started**: No real-work action
- **In progress**: Latest real-work action is running/pending
- **Awaiting review**: `implement` is done and the result mentions "push complete", "review", etc.
- **Awaiting deploy**: `review` / `self-review` is done and merge/deploy remains
- **Stalled**: Persistent failures (including `stale: ...`) or no activity for a long time
- **Blocked**: Stalled with a result that explicitly states a blocker (permission error, external dependency, etc.) that cannot be resolved independently
- **Likely complete**: A result satisfies all requirements but the task remains open

### 4. Summary

Present by project in a table:

| ID | Title | Age | Phase | Latest real-work |
|---|---|---|---|---|
| 157 | Implement feature A | 3d | Awaiting review | #815 implement done — implementation complete, pushed |
| 55 | Fix bug B | 5d | Not started | — |

### 5. Proposals

Propose cleanup actions via AskUserQuestion. **Every proposal MUST include a summary of the result that justifies the decision.**

Decision criteria:

- **Likely complete**: The real-work result satisfies the task requirements → propose `done`. Quote the evidence from the result.
- **Advance to next phase**: implementation done → review, review done → deploy, etc. → propose creating the next action. Reference the "next action" note in the result if present.
- **Not started**: No real-work action → ask whether to proceed; if yes, create an action, otherwise `archived`.
- **Stalled**: Persistent failures / long idle → quote the failure cause from the result and propose either changing approach or `archived`.
- **Blocked**: Quote the blocker from the result and propose a way to unblock.

Execute proposals only after user approval.
