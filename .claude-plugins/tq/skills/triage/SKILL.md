---
name: tq-triage
description: Inventory and organize open tasks - review status, propose cleanup, execute
argument-hint: "[project_name]"
allowed-tools: Bash(tq *), Bash(gh pr view *), Bash(find *), Read, AskUserQuestion, Skill(tq:create-action)
---

# tq triage

Inventory and organize open tasks.

## Steps

### 1. Collect (lightweight)

The raw `tq task list --status open` payload often exceeds 1MB. Fetch with `--jq` to extract only the fields needed for triage.

**MUST execute the following jq query verbatim — do not modify, simplify, or substitute custom field selections.** `latest_triage_note` is required by Step 3's skip rule; dropping it silently breaks the skip evaluation and forces re-decisions on already-triaged tasks.

```bash
tq task list --status open --jq '
  .[] | {
    id, project_id, title, updated_at,
    metadata_url: (.metadata // "{}" | try fromjson.url // null),
    latest_triage_note,
    latest: (
      .actions | sort_by(.created_at) | last
      | if . then {id, title, status, completed_at,
                    result_head: ((.result // "")[0:300])} else null end
    )
  }
'
```

`latest_triage_note` is the most recent `kind=triage_keep` note on the task, or `null`. When present it has `{reason, at, snooze_until?}`. It surfaces the previous "leave open" judgment so Step 3 can skip tasks whose situation has not changed.

Filter by `--project <id>` if `$ARGUMENTS` is given.

**Pre-flight declaration (MUST output before Step 2)**: After running the query, count the result and emit an assistant message in this exact form:

> Found N open tasks. M have prior `latest_triage_note` — Step 3 skip rule (including PR-merge override) will be evaluated for each.

Where `N` is the total task count and `M` is the count of tasks with `latest_triage_note != null`. Both numbers MUST be derived from the Step 1 query output. Do NOT proceed to Step 2 without emitting this declaration.

### 2. Project consistency check (before phase detection)

Detect tasks that landed in the wrong project due to auto-creation (e.g. `gh-ops:watch`). Run `tq project list` to get the project list with metadata, then infer the expected project from each task's `metadata_url` (pre-extracted in Step 1), title, and `latest.result_head`, and compare against the current `project_id`.

**Capture `dispatch_enabled` per project** from the same `tq project list` output. Build a `project_id → dispatch_enabled` map; it is reused in Steps 3, 5, and 6 to reason about focus. A project with `dispatch_enabled == false` ("unfocus") will NOT auto-dispatch its pending actions — they sit indefinitely unless manually dispatched or `dispatch_enabled` is flipped to `true`.

**Present mismatches and fix**: If mismatches are found, present them in the table below and confirm **one at a time** via `AskUserQuestion` (choices: `move` / `skip`). On each approval, run `tq task update <ID> --project <new_id>`. Never batch-approve.

| ID | Title | Current | Expected | Evidence |
|---|---|---|---|---|
| 420 | Respond to PR #55 | works | (example) | metadata.url: github.com/example/app/pull/55 |

Project moves are resolved here; subsequent steps use the updated `project_id`.

### 3. Phase detection

**PR-state pre-fetch** (runs first; the cache is read by the Phase criteria below, the skip rule, Step 4, and Step 6): For every task with a PR URL, run `gh pr view <url> --json state,mergedAt,mergeable,reviewDecision` calls **in parallel** (single message, multiple Bash calls). Cache the JSON by `task_id`. Tasks with no extractable PR URL skip this fetch — the PR-merge override below simply does not apply to them.

PR URL precedence per task:

1. Prefer `metadata_url` (extracted in Step 1). It is authoritative when set — the task was created with this specific PR in mind.
2. Otherwise, take the **first** match in `latest.result_head` from the regex `https://github\.com/[\w.-]+/[\w.-]+/pull/\d+`. The `[\w.-]+` form anchors org/repo segments and stops the URL cleanly at the PR number — no trailing `/files`, `#issuecomment-…`, or punctuation.

Now classify each task. Inspect `latest.status`, keywords in `latest.result_head`, and the cached PR state when available.

**Phase criteria**:

- **Not started**: `latest == null`.
- **In progress**: `latest.status ∈ {running, pending}`.
- **Awaiting review**: `latest.status == done` and result contains `push complete` / `review` / `PR opened`.
- **Awaiting deploy**: latest done-action is a review/self-review, merge/deploy remains.
- **Stalled**: Persistent failures (status `failed` multiple times, or `stale: ...` in result) or `updated_at` older than 14 days.
- **Blocked**: Stalled with a result that explicitly states a blocker (permission error, external dependency, etc.) that cannot be resolved independently.
- **Likely complete**: `state == MERGED` from the PR-state pre-fetch (above), or `latest.status == done` with `merged` / `done` / `complete` in the result.

**Focus qualifier**: When `latest.status ∈ {running, pending}` AND the task's project is unfocus (`dispatch_enabled == false`), append `(unfocus: manual dispatch required)` to the phase label. This surfaces the fact that a `pending` action in an unfocus project will not progress on its own.

**Triage skip rule** (after the phase is assigned): If `latest_triage_note != null`, evaluate whether the prior keep judgment still holds. The task is **skipped** from Step 6 (and shown in Step 5 as `triaged Nd ago: <reason>`) when **all** of the following are true:

- (a) `now - latest_triage_note.at < 7 days` (cooldown window).
- (b) No new action has completed (any `completed_at > latest_triage_note.at`) and no `task.status_changed` event since `latest_triage_note.at` for this task. Inspect the action list and `tq event list --entity task --id <id>` if needed.
- (c) `latest_triage_note.snooze_until` is unset, OR `now < latest_triage_note.snooze_until`.
- (d) The task either has no cached PR state, or `pr_state.mergedAt` is null, or `pr_state.mergedAt <= latest_triage_note.at` (see the **timestamp normalization** note below before comparing).

**PR-merge override**: A PR merged after the prior triage note (`pr_state.mergedAt > latest_triage_note.at`) breaks (d) and forces re-evaluation **even when (a) cooldown or (c) snooze would otherwise hold**. The rationale: a merged PR means the task should almost certainly be marked done, so neither the 7-day wait nor an explicit snooze is load-bearing once the PR has landed. This catches the case where a human merges a PR manually while a `self-review` action sits pending — without (d), the note keeps the task buried until the cooldown lapses even though the underlying work is finished.

**Out of scope for this override**: PR state transitions other than merge (CLOSED-without-merge, OPEN → APPROVED, mergeable conflict, etc.). They are still handled by clauses (a)/(b) if/when an action records them in tq, and by the Likely-complete phase rules in Step 6 once the next action runs. The override deliberately covers only the silent merge case.

**Timestamp normalization** (for clause (d)): `pr_state.mergedAt` from `gh pr view` is ISO 8601 with `T` separator and `Z` suffix (e.g. `2026-06-02T19:00:00Z`). `latest_triage_note.at` is SQLite TEXT with a space separator (e.g. `2026-06-02 19:00:00`). Direct string comparison is unsafe — `T` (0x54) sorts after `' '` (0x20), so a same-day note can wrongly compare less-than. Before evaluating clause (d), replace the space in `at` with `T` and append `Z` (or parse both as datetimes); only then compare.

If (c) is set and `now < snooze_until`, **skip even if (a) or (b) would re-evaluate** — explicit snooze wins. The PR-merge override (d) is the one exception: a post-note merge re-evaluates the task even with an active snooze. Otherwise, failing any of (a)/(b)/(c)/(d) means the task is re-evaluated normally in Step 6 and the prior reason is shown in option `description` for context.

**Recurring-task exclusion rule** (independent of the triage skip rule; applies even when `latest_triage_note == null`). Fetch the schedule map **once**, reuse in Steps 5/6:

```bash
tq schedule list --jq '.[] | {id, task_id, enabled}'
```

A task is **recurring** when it has a schedule-map entry OR any of its actions' `metadata` contains `schedule_id` (check `tq action list --task <id>` when the map is inconclusive). The map is authoritative for `enabled`.

**Exclude** a recurring task from Step 6 (Step 5 only, Phase annotated `(recurring)`) when **both**: `latest.status ∈ {done, pending}` (a `running` action is kept as In progress, not excluded) AND the task is recurring.

**Override — triage normally even when the exclude condition holds**: the backing schedule is absent from the map OR `enabled == false` (disabled/deleted — the recurring task may be orphaned and needs a human decision).

A recurring task whose `latest.status == failed` is never excluded in the first place (the exclude condition requires `done`/`pending`), so persistent scheduled-job failures always fall through to normal Step 6 triage by design — no extra override needed.

Excluded tasks are reported under the **recurring** category in Step 6's pre-report, not the triage-note list. If both rules exclude a task, the recurring category wins.

**Deep-dive condition**: If `latest.status == done` AND `len(result_head) == 300` (truncated) AND none of the keywords `push complete`, `review`, `merged`, `stale`, `blocked`, `failed`, `done` appear in `result_head` (case-insensitive), fetch the latest action's full result:

```bash
tq task get <ID> --jq '.actions | sort_by(.created_at) | last | {status, result}'
```

If multiple tasks need deep-dive, issue the `tq task get` calls in parallel (single message, multiple Bash calls). Skip deep-dive when `latest.status ∈ {running, pending}` or when the truncated head already contains a decisive keyword.

**Session-log fallback**: When the deep-dive still leaves `result` thin (failed action with a 1-line error, running action with empty `result`, or `len(result) < 100`) AND `metadata.claude_session_id` is non-empty, read the Claude Code session log to recover the missing detail:

```bash
SID=$(tq action get <id> --jq '.metadata | fromjson.claude_session_id // empty')
[ -n "$SID" ] && find ~/.claude/projects -name "$SID.jsonl" -print -quit
```

Use `Read` on the resolved path (last ~200 lines — the file may be large) and quote the latest few `type:"assistant"` entries (each embeds the response text plus any `tool_use` blocks in `message.content[]`) and any `type:"user"` entries carrying `tool_result` blocks, into 6-a Diagnosis. This does not replace the `tq task get` deep-dive — it runs in addition.

### 4. PR-state finalization

The Step 3 PR-state pre-fetch has already cached `gh pr view` JSON for every task with a PR URL — do not re-fetch here. Tasks without an extractable PR URL have no cache entry and skip this finalization (Step 3's keyword-based fallback in the Phase criteria already covers them).

Finalize classification using the cached state: `state == MERGED` → **Likely complete**. The cache remains available for Step 6.

### 5. Summary

Present tasks by project in a table. Mark each project's section header with its focus state (`focus` / `unfocus`) from the Step 2 map.

| ID | Title | Age | Phase | Latest action | Latest triage |
|---|---|---|---|---|---|
| 157 | Implement feature A | 3d | Awaiting review | #815 implement done — implementation complete, pushed | — |
| 302 | Refactor parser | 2d | In progress (unfocus: manual dispatch required) | #900 implement pending — queued, will not auto-dispatch | — |
| 55 | Fix bug B | 5d | Not started | — | 3d ago: awaiting PR review |
| 88 | Weekly rows-read check | 1d | Likely complete (recurring) | #940 run done — no regression, schedule #12 enabled | — |

The `Latest triage` column shows `Nd ago: <reason>` when `latest_triage_note` is present, otherwise `—`. Tasks skipped by the Step 3 triage skip rule still appear in this table but are excluded from Step 6. Tasks excluded by the Step 3 recurring-task exclusion rule also appear here, with `(recurring)` appended to the Phase column (e.g. `Likely complete (recurring)`), and are likewise excluded from Step 6.

Use the post-move `project_id` (tasks moved in Step 2 appear under their new project).

### 6. Proposals — per-task sequential triage (Rumelt's kernel of strategy)

**Pre-Step-6 skipped-task report (MUST output before any `AskUserQuestion`)**: Before starting the first task's 6-a Diagnosis, emit an assistant message listing every task excluded by the Step 3 triage skip rule, with the prior triage reason and timestamp. Use this exact form (one example with cooldown, one with snooze — pick the gating clause that matches each task):

> Skipping N tasks with valid prior triage notes:
> - Task #<id> (<title>) — `<reason>` (triaged Nd ago at YYYY-MM-DD; cooldown active)
> - Task #<id> (<title>) — `<reason>` (triaged Nd ago at YYYY-MM-DD; snooze_until: YYYY-MM-DD)

Each line MUST include the task `id`, `title`, the `latest_triage_note.reason` quoted verbatim in backticks, days since `latest_triage_note.at`, and the gating clause: `snooze_until: YYYY-MM-DD` when `latest_triage_note.snooze_until` is set, otherwise `cooldown active`. If no tasks are skipped, still emit `Skipping 0 tasks — no valid prior triage notes.` so the user can confirm the skip rule was evaluated. Do NOT issue the first `AskUserQuestion` without this report.

**Recurring-task exclusions (separate category, MUST also be emitted)**: list every task excluded by the Step 3 recurring-task exclusion rule. This is distinct from the triage-note list above — a task appears in only one. Use this exact form:

> Skipping M recurring tasks (healthy scheduled ops, latest status done/pending):
> - Task #<id> (<title>) — schedule #<sid> (enabled), latest action #<aid> <status>

If none, emit `Skipping 0 recurring tasks.`.

After the Step 5 summary and the skipped-task report, triage open tasks **one at a time, in order**. For each task, walk the three sub-steps below — Diagnosis, Guiding Policy, Coherent Actions — modeled on Richard Rumelt's *kernel of strategy* (*Good Strategy, Bad Strategy*): name the situation, choose a direction, then act coherently.

**Skip from this step**: tasks excluded by the **Step 3 triage skip rule** or the **Step 3 recurring-task exclusion rule** (both already enumerated in the report above), and tasks in the **In progress** phase (running — do not interrupt; they appear in Step 5 only). Project moves are already resolved in Step 2.

**MUST NOT** batch `AskUserQuestion` across tasks — task IDs and one-line summaries are not enough for a human to judge several cases in parallel. Complete 6-a → 6-b → 6-c → Step 7 execution for one task before starting the next task's 6-a.

#### 6-a. Diagnosis

In the **assistant message body** (not `AskUserQuestion`), present:

- Task ID, title, age (days since `updated_at`).
- Latest substantive action: ID, status, and a 1-3 line `result` quote (use a blockquote for the decisive lines).
- Phase classification from Step 3 + the specific evidence that decided it (which keyword in `result_head`, which `latest.status`, which PR `state` from the Step 3 cache).
- Phase-specific concern:
  - **Awaiting review / Awaiting deploy**: blockers, PR state from the Step 3 PR-state cache.
  - **Stalled / Blocked**: stall duration, unresolved obstacle.
  - **Likely complete**: completion evidence (PR merged, etc.).
  - **Not started**: probable reason for non-start.

#### 6-b. Guiding Policy

Continuing in the message body, state the recommended direction:

- Recommended action (`Mark done` / `Archive` / continue / `Create ... action` / `Manually dispatch` / `Enable dispatch`) and **why**.
- Counter-options ruled out and the reason.
- For unfocus-project tasks with stalled `pending` actions, justify whether to keep, manually dispatch, or enable dispatch — the user must know `pending` actions will not auto-dispatch.

Pick the 6-c options (2-4 per task) from this template:

| Phase | Options |
|---|---|
| Awaiting review | `Create review-request action` / `Create merge action` / `Mark done (already merged)` / `Leave open` |
| Awaiting deploy | `Create deploy action` / `Mark done` / `Leave open` |
| Stalled | `Create investigate-root-cause action` / `Change approach (new action)` / `Archive` / `Leave open` |
| Blocked | `Create unblock action` / `Archive` / `Leave open` |
| Likely complete | `Mark done` / `Create merge action` / `Leave open` (see PR-state rule below) |
| Not started | `Create first action` / `Archive` / `Leave open` |

**Likely complete — PR-state rule** (uses the Step 3 PR-state cache):

- `state == MERGED` → `Mark done` first, label `Mark done (Recommended)`.
- `state == OPEN` → `Create merge action` first, label `Create merge action (Recommended)`.

**Universal "leave open" options**: every phase may add `Leave open with note (keep)` and `Snooze N days` so the next triage run can skip the task. Plain `Leave open` remains for "no reason worth recording".

**Forward-motion default**: every phase except In progress MUST include at least one concrete forward-motion option (create next action, mark done, archive). Tasks must not sit as `Leave open` by default.

**Unfocus-aware options**: when the task's project is unfocus AND it has a `pending` action (or the proposal would otherwise be `Leave open`), the option set MUST include both, with the unfocus state stated in the option `description`:

- `Manually dispatch pending action` → runs `tq action dispatch <action_id>` for the waiting action.
- `Enable dispatch and batch-run` → runs `tq project update <project_id> --dispatch-enabled true` so all pending actions in that project drain automatically.

**Likely complete with pending follow-ups**: when proposing `Mark done` while `pending` actions remain, distinguish in the option `description`:

- Pending in an **unfocus** project → likely stale (queued but unreachable). Recommend `Mark done` and cancel/rework the leftovers separately.
- Pending in a **focus** project → genuine follow-up in flight. Prefer `Leave open` until they complete, or cancel them explicitly before `Mark done`.

#### 6-c. Coherent Actions

Issue **one** `AskUserQuestion` for **this task only**:

- 2-4 options drawn from the 6-b template, shaped by the 6-a / 6-b context.
- Place the recommended option first; append `(Recommended)` to its label.
- Per-option `description` MUST contain the decision material:
  - 1-2 line summary of the latest `result`.
  - For PR-related tasks: PR number + state from the Step 3 PR-state cache.
  - Days since last update (`updated_at` vs today).

**Action instruction quality**: when the user picks `Create ... action`, invoke `/tq:create-action` with a `task_id` and an instruction that quotes the specific next step from the previous `result` (e.g. "Request review on PR #XXX" — not a vague "request review"). Include relevant URLs, IDs, and any `next action` note from the prior result. Do not call `tq action create` directly.

After the user approves, immediately run the corresponding Step 7 command for the chosen option. Only once that command has completed (or `Leave open` etc. has been recorded) do you move on to the **next task's 6-a**. Do not present another task's diagnosis before the current task's Step 7 reflection finishes.

### 7. Execute

Execute each approved option immediately:

- `Mark done` → `tq task update <ID> --status done --note '<1-line why it is complete, cite the result evidence>'` (`--note` is required alongside `--status`).
- `Archive` → `tq task update <ID> --status archived --note '<1-line reason, e.g. "stalled 30d, no path forward">'`.
- `Create ... action` → invoke `/tq:create-action` (task_id + instruction). Do not call `tq action create` directly.
- `Manually dispatch pending action` → `tq action dispatch <action_id>` for the pending action identified in Step 3.
- `Enable dispatch and batch-run` → `tq project update <project_id> --dispatch-enabled true`. Report the project name to the user so they know which project now auto-dispatches.
- `Leave open with note (keep)` → ask the user for a one-line reason, then `tq task note <ID> --kind triage_keep --reason '<reason>'`.
- `Snooze N days` → ask the user for the snooze duration (number of days, or an explicit `YYYY-MM-DD` target date). Compute `snooze_until` and run `tq task note <ID> --kind triage_keep --reason '<reason>' --metadata '{"snooze_until":"YYYY-MM-DD"}'`.
- `Leave open` → no-op.

If an execute fails, record the error, report it to the user, and continue with the remaining batch. After all rounds of all phases complete, triage ends.
