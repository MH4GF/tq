---
description: Inventory and organize open tasks - review status, propose cleanup, execute
argument-hint: "[project_name]"
---

# tq triage

Inventory and organize open tasks.

## Steps

### 1. Collect (lightweight)

The raw `tq task list --status open` payload often exceeds 1MB. Fetch with `--jq` to extract only the fields needed for triage:

```bash
tq task list --status open --jq '
  .[] | {
    id, project_id, title, updated_at,
    metadata_url: (.metadata // "{}" | try fromjson.url // null),
    latest: (
      .actions | sort_by(.created_at) | last
      | if . then {id, title, status, completed_at,
                    result_head: ((.result // "")[0:300])} else null end
    )
  }
'
```

Filter by `--project <id>` if `$ARGUMENTS` is given.

### 2. Project consistency check (before phase detection)

Detect tasks that landed in the wrong project due to auto-creation (e.g. `gh-ops:watch`). Run `tq project list` to get the project list with metadata, then infer the expected project from each task's `metadata_url` (pre-extracted in Step 1), title, and `latest.result_head`, and compare against the current `project_id`.

**Present mismatches and fix**: If mismatches are found, present them in the table below and confirm **one at a time** via `AskUserQuestion` (choices: `move` / `skip`). On each approval, run `tq task update <ID> --project <new_id>`. Never batch-approve.

| ID | Title | Current | Expected | Evidence |
|---|---|---|---|---|
| 420 | Respond to PR #55 | works | (example) | metadata.url: github.com/example/app/pull/55 |

Project moves are resolved here; subsequent steps use the updated `project_id`.

### 3. Phase detection

Classify each task from the Step 1 output. Inspect `latest.status` and keywords in `latest.result_head`.

**Phase criteria**:

- **Not started**: `latest == null`.
- **In progress**: `latest.status ∈ {running, pending}`.
- **Awaiting review**: `latest.status == done` and result contains `push complete` / `review` / `PR opened`.
- **Awaiting deploy**: latest done-action is a review/self-review, merge/deploy remains.
- **Stalled**: Persistent failures (status `failed` multiple times, or `stale: ...` in result) or `updated_at` older than 14 days.
- **Blocked**: Stalled with a result that explicitly states a blocker (permission error, external dependency, etc.) that cannot be resolved independently.
- **Likely complete**: `state == MERGED` from the PR-state pre-fetch (below), or `latest.status == done` with `merged` / `done` / `complete` in the result.

**Deep-dive condition**: If `latest.status == done` AND `len(result_head) == 300` (truncated) AND none of the keywords `push complete`, `review`, `merged`, `stale`, `blocked`, `failed`, `done` appear in `result_head` (case-insensitive), fetch the latest action's full result:

```bash
tq task get <ID> --jq '.actions | sort_by(.created_at) | last | {status, result}'
```

If multiple tasks need deep-dive, issue the `tq task get` calls in parallel (single message, multiple Bash calls). Skip deep-dive when `latest.status ∈ {running, pending}` or when the truncated head already contains a decisive keyword.

### 4. PR-state pre-fetch

For tasks classified as Awaiting review / Awaiting deploy / Likely complete, collect all PR URLs from `metadata_url` or the latest result and run `gh pr view <url> --json state,mergedAt,mergeable,reviewDecision` calls **in parallel** (single message, multiple Bash calls).

Finalize classification using the state: `state == MERGED` → **Likely complete**. Cache the JSON for Step 6.

### 5. Summary

Present tasks by project in a table:

| ID | Title | Age | Phase | Latest action |
|---|---|---|---|---|
| 157 | Implement feature A | 3d | Awaiting review | #815 implement done — implementation complete, pushed |
| 55 | Fix bug B | 5d | Not started | — |

Use the post-move `project_id` (tasks moved in Step 2 appear under their new project).

### 6. Proposals — per-phase batching via AskUserQuestion

**Batching rule**: Group tasks by phase. Within a phase, issue `AskUserQuestion` with at most 4 questions per call (one question per task). If the phase has more than 4 tasks, issue multiple rounds. Project moves are already resolved in Step 2 and are out of scope here. Tasks in **In progress** are shown in the Summary but skipped here (they are running — do not interrupt).

**Per-option `description` must contain** the material a user needs to decide without scrolling back:

- 1-2 line summary of the latest `result`.
- For PR-related tasks: PR number + state from the Step 4 PR-state cache.
- Days since last update (`updated_at` vs today).

**Phase-specific option templates** (pick 2-4 per task):

| Phase | Options |
|---|---|
| Awaiting review | `Create review-request action` / `Create merge action` / `Mark done (already merged)` / `Leave open` |
| Awaiting deploy | `Create deploy action` / `Mark done` / `Leave open` |
| Stalled | `Create investigate-root-cause action` / `Change approach (new action)` / `Archive` / `Leave open` |
| Blocked | `Create unblock action` / `Archive` / `Leave open` |
| Likely complete | `Mark done` / `Create merge action` / `Leave open` (see PR-state rule below) |
| Not started | `Create first action` / `Archive` / `Leave open` |

**Likely complete — PR-state rule** (uses cache from Step 4):

- `state == MERGED` → put `Mark done` first with label `Mark done (Recommended)`.
- `state == OPEN` → put `Create merge action` first with label `Create merge action (Recommended)`.

**Forward-motion default**: For every phase except In progress, at least one option MUST be a concrete forward-movement action (create next action, mark done, archive). Do not let tasks sit as "Leave open" by default.

**Action instruction quality**: When the user picks a `Create ... action` option, invoke `/tq:create-action` with a task_id and an instruction that quotes the specific next step from the previous `result` (e.g. "Request review on PR #XXX" — not a vague "request review"). Include relevant URLs, IDs, and the `next action` note from the prior result.

**Recommended marker**: Per `AskUserQuestion` spec, place the recommended option first and append `(Recommended)` to its label.

### 7. Execute

Execute each approved option immediately:

- `Mark done` → `tq task update <ID> --status done --note '<1-line why it is complete, cite the result evidence>'` (`--note` is required alongside `--status`).
- `Archive` → `tq task update <ID> --status archived --note '<1-line reason, e.g. "stalled 30d, no path forward">'`.
- `Create ... action` → invoke `/tq:create-action` (task_id + instruction). Do not call `tq action create` directly.
- `Leave open` → no-op.

If an execute fails, record the error, report it to the user, and continue with the remaining batch. After all rounds of all phases complete, triage ends.
