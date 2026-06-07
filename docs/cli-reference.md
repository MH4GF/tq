# CLI Reference

All list commands output JSON. Use the `--jq` flag to filter output inline.

For detailed help on any command, run `tq <command> <subcommand> --help`.

## project

| Command | Description |
|---------|-------------|
| `tq project create <NAME> <WORK_DIR>` | Create a new project |
| `tq project list` | List projects (JSON output) |
| `tq project update <ID>` | Update a project |
| `tq project delete <ID>` | Delete a project |

### `tq project create`

```
tq project create <NAME> <WORK_DIR> [--meta <JSON>]
```

- `--meta` — JSON metadata (e.g. `{"team":"platform"}`)

### `tq project update`

```
tq project update <ID> [--work-dir <PATH>] [--dispatch-enabled <true/false>]
```

- `--work-dir` — Set the working directory
- `--dispatch-enabled` — Enable or disable dispatch

### `tq project delete`

```
tq project delete <ID> [--cascade]
```

- `--cascade` — Delete all tasks, actions, and schedules belonging to this project. Completion-dependency edges in *other* projects that pointed at a deleted task/action are also purged, so their dependents are no longer blocked by the removed work.

### `tq project list`

```
tq project list [--jq <EXPR>] [--limit <N>]
```

- `--jq` — Filter JSON output (fields: `id`, `name`, `work_dir`, `metadata`, `dispatch_enabled`, `created_at`)
- `--limit` — Limit number of results (0 = no limit)

## task

| Command | Description |
|---------|-------------|
| `tq task create <TITLE> --project <ID>` | Create a new task |
| `tq task list` | List tasks (JSON output, includes latest 10 nested actions) |
| `tq task get <ID>` | Get a task by ID (JSON output, includes latest 10 nested actions and status_history) |
| `tq task update <ID>` | Update a task |
| `tq task note <ID>` | Record a free-form note on a task without changing its status |

### `tq task create`

```
tq task create <TITLE> --project <ID> [--work-dir <PATH>] [--meta <JSON>]
```

- `--project` — Project ID (**required**)
- `--work-dir` — Working directory (defaults to project's work_dir)
- `--meta` — JSON metadata (e.g. `{"url":"https://..."}`)

### `tq task list`

```
tq task list [--project <ID>] [--status <STATUS>] [--jq <EXPR>] [--limit <N>]
```

- `--project` — Filter by project ID
- `--status` — Filter by status (`open`, `done`, `archived`)
- `--jq` — Filter JSON output (fields: `id`, `project_id`, `title`, `metadata`, `status`, `work_dir`, `created_at`, `updated_at`, `actions`, `latest_triage_note`)
- `--limit` — Limit number of results

`actions` contains at most the **latest 10** actions per task (ascending by id, so `actions[-1]` is the most recent). Use `tq action list --task <ID>` for the full history.

`latest_triage_note` is the most recent `kind=triage_keep` note on the task, or `null`. When present it has `{reason, at, snooze_until?}` (the `snooze_until` key is surfaced from the note's `metadata` when set). It is intended for the `/tq:triage` skill to skip tasks whose situation has not changed since the prior keep judgment.

### `tq task get`

```
tq task get <ID> [--jq <EXPR>]
```

- `--jq` — Filter JSON output (fields: `id`, `project_id`, `title`, `metadata`, `status`, `work_dir`, `created_at`, `updated_at`, `actions`, `status_history`, `notes`)

`actions` contains at most the **latest 10** actions (ascending by id, so `actions[-1]` is the most recent). Use `tq action list --task <ID>` for the full history.

`status_history` is an array of status transitions derived from `task.status_changed` events. Each entry has `{from, to, at}` plus optional `reason` (set when `tq task update --note` was used).

`notes` is an array of free-form notes derived from `task.note` events (recorded via `tq task note`). Each entry has `{kind, reason, at}` plus optional `metadata`. Notes are independent of `status_history`.

### `tq task update`

```
tq task update <ID> [--status <STATUS>] [--project <ID>] [--work-dir <PATH>] [--meta <JSON>] [--note <TEXT>]
```

At least one flag is required. `--status` and `--note` must be specified together.

- `--status` — New status (`open`, `done`, `archived`)
- `--project` — Project ID
- `--work-dir` — Working directory
- `--meta` — JSON metadata to merge
- `--note` — Reason for the status change (**required when `--status` is given**; recorded in the event log)

### `tq task note`

```
tq task note <ID> --kind <KIND> --reason <TEXT> [--metadata <JSON>]
```

Record a status-independent annotation on a task. Notes appear in `tq task get` under `notes` and do not modify `status` or `status_history`.

- `--kind` — Note kind, free-form string (e.g. `triage_keep`, `observation`, `blocker`) (**required**)
- `--reason` — One-line explanation (**required**)
- `--metadata` — JSON object with kind-specific extras (e.g. `{"snooze_until":"2026-05-02"}`)

Notes with `kind=triage_keep` are surfaced on `tq task list` as `latest_triage_note` and consumed by the `/tq:triage` skill to skip tasks whose situation has not changed.

## action

| Command | Description |
|---------|-------------|
| `tq action create <INSTRUCTION> --task <ID> --title <TITLE>` | Create an action |
| `tq action list` | List actions |
| `tq action get <ID>` | Get an action by ID (JSON output) |
| `tq action update <ID>` | Update an action |
| `tq action done <ID> [RESULT]` | Mark action as done |
| `tq action fail <ID> [REASON]` | Mark action as failed when the goal could not be achieved |
| `tq action cancel <ID> [REASON]` | Cancel a pending, running, dispatched, or failed action |
| `tq action attach <ID>` | Attach to a running action (claude agent view) |
| `tq action reset <ID>` | Reset a failed or cancelled action to pending |
| `tq action dispatch <ID>` | Dispatch an action immediately (skip queue) |
| `tq action resume <ID>` | Resume the claude session of a closed action |
| `tq action prompt <ID>` | Render the wrapped claude prompt for an action |

### `tq action create`

```
tq action create <INSTRUCTION> --task <ID> --title <TITLE> [--meta <JSON>] [--status <STATUS>] [--after <TIME>] [--work-dir <PATH>] [--blocked-by-action <ID>]... [--blocked-by-task <ID>]...
```

- `--task` — Task ID (**required**). Rejected if the task status is `done` or `archived`; reopen with `tq task update <ID> --status open` first if intentional.
- `--title` — Action title (**required**, max 100 chars)
- `--meta` — JSON metadata for dispatch control:
  - `mode` — `"interactive"`, `"noninteractive"`, or `"remote"`. `interactive` and `noninteractive` both launch via `claude --bg` so the action appears in `claude agents`; the difference is which slot pool the action consumes. `remote` dispatches via `claude --remote` for cloud execution. When omitted, the global default from `tq config get default_mode` is stamped into the action's metadata; if that is also unset it falls back to `"interactive"`. An explicit value here always wins. Any other value is rejected — pass Claude permission-mode (`auto`, `plan`, `acceptEdits`, …) via `claude_args` instead.
  - `claude_args` — Additional CLI arguments for claude (JSON array of strings, e.g. `["--permission-mode","plan","--worktree","--max-turns","5"]`)
  - `executor` — `"local"` or `"cloud"`. Records where the action's claude session is actually running (orthogonal to `mode`). The reaper skips actions marked `executor=cloud` since local session-log liveness checks do not apply. Auto-stamped to `cloud` when `--status running` is passed from a Claude Code cloud session (`CLAUDE_CODE_REMOTE=true`); also stamped by the `SessionStart` hook in cloud sessions launched via tq dispatch. Explicit values in `--meta` are preserved.
- `--status` — Initial status (default: `pending`)
- `--after` — Dispatch after this time (`YYYY-MM-DD HH:MM`, local timezone)
- `--work-dir` — Working directory override for this action only (does not modify the parent task's `work_dir`). Dispatch resolves the effective directory as **action.work_dir → task.work_dir → project.work_dir → `.`**. When the override path does not exist on disk at dispatch time, tq logs a warning and falls back to the task chain *without clearing the override*, so the explicit user intent is preserved. Resume follow-ups inherit this `work_dir`.
- `--blocked-by-action` / `--blocked-by-task` — Completion dependencies (repeatable, AND). The action stays `pending` until **every** blocker reaches a successful terminal state (`action`=`done`, `task`=`done`/`archived`); the queue worker dispatches it automatically the moment the last blocker completes. A blocker that ends `failed`/`cancelled` blocks the action **forever** by design — rescue it with the `/tq:dep-triage` skill (or `tq action update --clear-deps`). Deleting the blocker task/action (including via `tq project delete --cascade`) purges the edge, which also unblocks the dependent. Combines with `--after`: both the time gate and all dependencies must be satisfied. `tq action dispatch <ID>` is a manual override that bypasses this gate.

### `tq action list`

```
tq action list [--task <ID>] [--status <STATUS>] [--jq <EXPR>] [--limit <N>]
```

- `--task` — Filter by task ID
- `--status` — Filter by status (`pending`, `running`, `dispatched`, `done`, `failed`, `cancelled`)
- `--jq` — Filter JSON output (fields: `id`, `title`, `task_id`, `metadata`, `status`, `result`, `tmux_session`, `tmux_window`, `dispatch_after`, `work_dir`, `started_at`, `completed_at`, `created_at`, `blocked_by`)
- `--limit` — Limit number of results

### `tq action get`

```
tq action get <ACTION_ID> [--jq <EXPR>]
```

Print a single action as JSON.

- `--jq` — Filter JSON output using a jq expression (fields: `id`, `title`, `task_id`, `metadata`, `status`, `result`, `tmux_session`, `tmux_window`, `dispatch_after`, `work_dir`, `started_at`, `completed_at`, `created_at`, `blocked_by`)

### `tq action done`

```
tq action done <ACTION_ID> [RESULT]
```

Mark a non-terminal action (pending, running, or dispatched) as done. Calling `done` on an action that is already `done`, `failed`, or `cancelled` returns an error that includes a status-specific recovery hint: for a false-positive `failed`/`cancelled` (e.g. stale heartbeat or timeout), `tq action reset <ID>` then re-run `tq action done`; for an already-`done` action, amend the result with `tq action update <ID> --result`.

RESULT is free-form text. Recommended structure:

- **outcome** — What changed (concrete deliverables)
- **decisions** — What was decided and why
- **artifacts** — PR numbers, file paths, commit SHAs, URLs
- **remaining** — Unfinished work, known issues, follow-up needed. Each entry that carries future work must reference a filed follow-up action (`- <what remains> → #<id>`); the `/tq:done` skill enforces this so the work stays visible to `tq action list` and `/tq:triage`

### `tq action fail`

```
tq action fail <ACTION_ID> [REASON]
```

Mark a non-terminal action (pending, running, or dispatched) as failed when the goal could not be achieved despite genuine effort. Use this for missing permissions, broken environment, external API outages, or other blockers. Failed actions can be reset to pending with `tq action reset` for retry.

Distinction:
- **done** — work completed successfully
- **fail** — work attempted but blocked (retryable)
- **cancel** — work intentionally aborted (not needed, superseded)

REASON is free-form text. Recommended structure (same as `done`):

- **outcome** — What could not be achieved (the concrete blocker)
- **decisions** — What was tried and why it did not work
- **artifacts** — Partial PRs, files, log excerpts, error messages
- **remaining** — What is needed to unblock (env fix, external response, retry conditions). Retry/alternative work that a future session must do should reference a filed follow-up action (`- <what to retry> → #<id>`); the `/tq:failed` skill enforces this

### `tq action cancel`

```
tq action cancel <ACTION_ID> [REASON]
```

REASON serves as feedback for improving classification logic. Record why the action was unnecessary and how classification could be improved. If cancelling surfaces residual work someone must still do, file it (`tq action create … --task <id>`) and reference it in the reason (`next: <what to do> → #<id>`); the `/tq:cancel` skill enforces this so the work stays visible.

### `tq action update`

```
tq action update <ID> [--title <TITLE>] [--task <ID>] [--meta <JSON>] [--work-dir <PATH>] [--result <TEXT>] [--blocked-by-action <ID>]... [--blocked-by-task <ID>]... [--clear-deps]
```

`--title` / `--task` / `--work-dir` can only be changed on `pending` or `failed` actions; running, dispatched, done, or cancelled actions are rejected.

`--meta` and `--result` are allowed on `pending`, `failed`, `done`, or `cancelled` actions so post-execution observability fields (e.g. `claude_session_id`, `executor`) can be backfilled after the worker has marked the action terminal. Running/dispatched remain rejected because they are in-flight — use `tq action done`/`fail` instead.

- `--work-dir` — Override or clear the action-level working directory. Pass an empty string (`--work-dir ""`) to clear.
- `--meta` — Merge JSON object into existing metadata (existing keys are overwritten; other keys preserved). Allowed on `pending`, `failed`, `done`, or `cancelled`. Typical backfill: `tq action update <ID> --meta '{"claude_session_id":"<uuid>"}'` so `tq action resume <ID>` becomes viable for an older action whose session id was not auto-recorded.
- `--result` — Amend the recorded result. Same status whitelist as `--meta`. This is the recovery path for a result wrongly committed on an already-`done` action.
- `--blocked-by-action` / `--blocked-by-task` — Append completion dependencies (repeatable; same semantics as `tq action create`). Allowed on `pending` or `failed` actions only.
- `--clear-deps` — Remove all dependencies first. Use alone to unblock a forever-blocked action, or with `--blocked-by-*` to replace the dependency set (e.g. re-point to a resumed blocker). This is the primary recovery path out of a failed-blocker dead end.

### `tq action dispatch`

```
tq action dispatch <ACTION_ID>
```

Manual override: dispatches the action immediately, **bypassing the completion-dependency gate** (so an action whose blockers are unsatisfied — including blocked-forever ones — is dispatched anyway). The automatic queue worker still respects dependencies.

### `tq action resume`

```
tq action resume <ACTION_ID> [--message <TEXT>] [--mode <MODE>]
```

Create a new action that resumes the claude session of a previously completed/failed/cancelled action via `claude --resume <claude_session_id>`. The new action is linked to the parent through `metadata.parent_action_id` and `metadata.is_resume = true`.

The parent must be in `failed` / `cancelled` / `done` status and have a non-empty `claude_session_id` in metadata. Only the `claude_session_id` is inherited — other `claude_args` (`--worktree`, `--permission-mode`, etc.) are dropped because the resumed claude session restores its own context.

`claude_session_id` is populated by the queue worker's bg reaper, which reads `~/.claude/jobs/<short>/state.json` and back-fills the daemon-recorded `sessionId` field. Cloud-executed actions still rely on the tq Claude Code plugin's `SessionStart` hook because they retain `TQ_ACTION_ID` in their environment. For pre-existing actions without a recorded session id, backfill via `tq action update <ID> --meta '{"claude_session_id":"<uuid>"}'`.

- `--message` — Additional instruction passed as the new prompt (default: `"Continue the previous session."`)
- `--mode` — Execution mode: `interactive` | `noninteractive` | `remote` (default: parent action's mode). Any other value is rejected.

### `tq action prompt`

```text
tq action prompt <ACTION_ID>
```

Render the wrapped claude prompt (instruction + tq action context postamble) for an action and write it to stdout, ending with exactly one trailing LF. Output is byte-identical to the prompt the workers pass to `claude --bg` / `claude --remote`.

### `tq action attach`

```
tq action attach <ACTION_ID>
```

Exec `claude attach <daemon_short>` so the background session takes over the current terminal. Returns an error if the action is `mode=remote` (no local daemon session to attach to) or if the dispatch has not yet recorded `metadata.daemon_short`.

### `tq action reset`

```
tq action reset <ACTION_ID>
```

Reset a `failed` or `cancelled` action back to `pending` so it can be re-dispatched. The action's `started_at`, `tmux_session`, and `tmux_window` fields are cleared (the tmux columns are vestigial — kept for historical rows and never set by new dispatches).

Only `failed` and `cancelled` actions can be reset. `pending` and `done` actions return an error; `running` and `dispatched` actions are also rejected to prevent spawning a duplicate worker — cancel or fail them first, then reset.

## schedule

| Command | Description |
|---------|-------------|
| `tq schedule create` | Create a new schedule |
| `tq schedule list` | List schedules (JSON output) |
| `tq schedule get <ID>` | Get a schedule by ID (JSON output) |
| `tq schedule update <ID>` | Update a schedule |
| `tq schedule delete <ID>` | Delete a schedule |
| `tq schedule enable <ID>` | Enable a schedule |
| `tq schedule disable <ID>` | Disable a schedule |

### `tq schedule create`

```
tq schedule create --instruction <TEXT> --task <ID> --cron <EXPR> [--title <TITLE>] [--meta <JSON>]
```

- `--instruction` — Instruction text (**required**)
- `--task` — Task ID (**required**)
- `--cron` — Cron expression, 5-field format, evaluated in local timezone (**required**, e.g. `"0 9 * * *"`)
- `--title` — Schedule title (defaults to instruction)
- `--meta` — JSON metadata for dispatch control (same keys as `action create`)

### `tq schedule list`

```
tq schedule list [--jq <EXPR>] [--limit <N>]
```

- `--jq` — Filter JSON output (fields: `id`, `task_id`, `instruction`, `title`, `cron_expr`, `metadata`, `enabled`, `last_run_at`, `last_error`, `created_at`)
- `last_error` is `null` while the schedule is healthy; it is populated when an action could not be created (e.g. malformed or invalid metadata) and cleared on the next successful run.
- `--limit` — Limit number of results

### `tq schedule get`

```
tq schedule get <ID> [--jq <EXPR>]
```

- `--jq` — Filter JSON output (same fields as `tq schedule list`)

### `tq schedule update`

```
tq schedule update <ID> [--cron <EXPR>] [--title <TITLE>] [--task <ID>] [--instruction <TEXT>] [--meta <JSON>]
```

## event

| Command | Description |
|---------|-------------|
| `tq event list` | List events |

### `tq event list`

```
tq event list [--entity <TYPE>] [--id <ID>] [--jq <EXPR>] [--limit <N>]
```

- `--entity` — Filter by entity type (`action`, `task`, `project`, `schedule`)
- `--id` — Filter by entity ID (requires `--entity`)
- `--jq` — Filter JSON output (fields: `id`, `entity_type`, `entity_id`, `event_type`, `payload`, `created_at`)
- `--limit` — Number of recent events to show (default: 50)

## search

```
tq search <KEYWORD> [--project <ID>] [--jq <EXPR>]
```

Full-text search across task titles, task metadata, task status change reasons, action titles, action results, and action metadata. Output is JSON.

Substring search is backed by an FTS5 trigram index, so keywords shorter than 3 characters (including 2-character CJK terms) return no results.

- `--project` — Filter by project ID (default: 0 = all projects)
- `--jq` — Filter JSON output (fields: `entity_type`, `entity_id`, `task_id`, `project_id`, `field`, `snippet`, `status`, `created_at`)

## config

Global key-value settings stored in the DB, so configuration travels with libsql/Turso endpoints rather than a local file.

```
tq config set <KEY> <VALUE>
tq config get <KEY>
tq config list [--jq <EXPR>]
```

| Key | Description |
|-----|-------------|
| `default_mode` | Default execution mode (`interactive`, `noninteractive`, `remote`) stamped into a new action's metadata when `tq action create --meta` does not specify one. An explicit `--meta '{"mode":...}'` always overrides it. When unset, actions fall back to `interactive` at dispatch time. |

Only recognized keys are accepted; unknown keys and invalid values are rejected. `tq config get` prints an empty line when the key is unset. `tq config list` outputs JSON (fields: `key`, `value`).

## completion

| Command | Description |
|---------|-------------|
| `tq completion bash` | Generate the autocompletion script for bash |
| `tq completion zsh` | Generate the autocompletion script for zsh |
| `tq completion fish` | Generate the autocompletion script for fish |
| `tq completion powershell` | Generate the autocompletion script for powershell |

Each subcommand accepts `--no-descriptions` to disable completion descriptions. See `tq completion <shell> --help` for installation instructions.

## ui

```
tq ui [--max-interactive <N>] [--max-noninteractive <N>] [--poll <DURATION>]
```

- `--max-interactive` — Maximum concurrent sessions in the interactive slot pool (default: `3`)
- `--max-noninteractive` — Maximum concurrent sessions in the noninteractive slot pool (default: `5`)
- `--poll` — Queue worker poll interval (default: `10s`)

The two caps are independent slots. Both pools launch local actions via `claude --bg`; the pool choice is the only difference between `mode=interactive` and `mode=noninteractive`. See [`docs/dispatch.md`](dispatch.md) for the concurrency model.

Keyboard shortcuts in the task list (the help bar surfaces each key only when the cursor is on an eligible row):

- `d` — Dispatch the selected `pending` action immediately, equivalent to `tq action dispatch <ID>` (bypasses the completion-dependency gate).
- `r` — Resume the selected action by creating a new action that continues its Claude session. Only available on terminal actions (`done` / `failed` / `cancelled`) whose metadata carries a `claude_session_id`.
- `f` — Toggle `dispatch_enabled` (focus) on the selected project, equivalent to `tq project update <ID> --dispatch-enabled true|false`. Only available when the cursor is on a project header row.
