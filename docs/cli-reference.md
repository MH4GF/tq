# CLI Reference

All list commands output JSON. Use the `--jq` flag to filter output inline.

For detailed help on any command, run `tq <command> <subcommand> --help`.

## project

| Command | Description |
|---------|-------------|
| `tq project create <NAME> <WORK_DIR>` | Register a project |
| `tq project list` | List projects (JSON) |
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

- `--cascade` — Delete all tasks, actions, and schedules belonging to this project

### `tq project list`

```
tq project list [--jq <EXPR>] [--limit <N>]
```

- `--jq` — Filter JSON output (fields: `id`, `name`, `work_dir`, `metadata`, `dispatch_enabled`, `created_at`)
- `--limit` — Limit number of results (0 = no limit)

## task

| Command | Description |
|---------|-------------|
| `tq task create <TITLE> --project <ID>` | Create a task |
| `tq task list` | List tasks with latest 10 nested actions (JSON) |
| `tq task get <ID>` | Get a task by ID with latest 10 nested actions and status_history (JSON) |
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
| `tq action list` | List actions (JSON) |
| `tq action get <ID>` | Get an action by ID (JSON) |
| `tq action update <ID>` | Update an action |
| `tq action done <ID> [RESULT]` | Mark action as done |
| `tq action fail <ID> [REASON]` | Mark action as failed when the goal could not be achieved |
| `tq action cancel <ID> [REASON]` | Cancel an action |
| `tq action attach <ID>` | Attach to a running action's tmux window |
| `tq action reset <ID>` | Reset a failed or cancelled action to pending |
| `tq action dispatch <ID>` | Dispatch immediately (skip queue) |

### `tq action create`

```
tq action create <INSTRUCTION> --task <ID> --title <TITLE> [--meta <JSON>] [--status <STATUS>] [--after <TIME>]
```

- `--task` — Task ID (**required**)
- `--title` — Action title (**required**, max 100 chars)
- `--meta` — JSON metadata for dispatch control:
  - `mode` — `"interactive"` (default), `"noninteractive"`, `"remote"`
  - `claude_args` — Additional CLI arguments for claude (JSON array of strings, e.g. `["--permission-mode","plan","--worktree","--max-turns","5"]`)
- `--status` — Initial status (default: `pending`)
- `--after` — Dispatch after this time (`YYYY-MM-DD HH:MM`, local timezone)

### `tq action list`

```
tq action list [--task <ID>] [--status <STATUS>] [--jq <EXPR>] [--limit <N>]
```

- `--task` — Filter by task ID
- `--status` — Filter by status (`pending`, `running`, `dispatched`, `done`, `failed`, `cancelled`)
- `--jq` — Filter JSON output (fields: `id`, `title`, `task_id`, `metadata`, `status`, `result`, `session_id`, `dispatch_after`, `started_at`, `completed_at`, `created_at`)
- `--limit` — Limit number of results

### `tq action done`

```
tq action done <ACTION_ID> [RESULT]
```

Mark a non-terminal action (pending or running) as done. Calling `done` on an action that is already `done`, `failed`, or `cancelled` returns an error.

RESULT is free-form text. Recommended structure:

- **outcome** — What changed (concrete deliverables)
- **decisions** — What was decided and why
- **artifacts** — PR numbers, file paths, commit SHAs, URLs
- **remaining** — Unfinished work, known issues, follow-up needed

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
- **remaining** — What is needed to unblock (env fix, external response, retry conditions)

### `tq action cancel`

```
tq action cancel <ACTION_ID> [REASON]
```

REASON serves as feedback for improving classification logic. Record why the action was unnecessary and how classification could be improved.

### `tq action update`

```
tq action update <ID> [--title <TITLE>] [--task <ID>] [--meta <JSON>]
```

### `tq action dispatch`

```
tq action dispatch <ACTION_ID> [--session <NAME>]
```

- `--session` — Target tmux session name (default: `main`)

## schedule

| Command | Description |
|---------|-------------|
| `tq schedule create` | Create a schedule |
| `tq schedule list` | List schedules (JSON) |
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
- `--cron` — Cron expression, 5-field format (**required**, e.g. `"0 9 * * *"`)
- `--title` — Schedule title (defaults to instruction)
- `--meta` — JSON metadata for dispatch control (same keys as `action create`)

### `tq schedule list`

```
tq schedule list [--jq <EXPR>] [--limit <N>]
```

- `--jq` — Filter JSON output (fields: `id`, `task_id`, `instruction`, `title`, `cron_expr`, `metadata`, `enabled`, `last_run_at`, `created_at`)
- `--limit` — Limit number of results

### `tq schedule update`

```
tq schedule update <ID> [--cron <EXPR>] [--title <TITLE>] [--task <ID>] [--instruction <TEXT>] [--meta <JSON>]
```

## event

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

- `--project` — Filter by project ID (default: 0 = all projects)
- `--jq` — Filter JSON output (fields: `entity_type`, `entity_id`, `task_id`, `project_id`, `field`, `snippet`, `status`, `created_at`)

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
tq ui [--max-interactive <N>] [--poll <DURATION>] [--session <NAME>]
```

- `--max-interactive` — Maximum concurrent interactive sessions (default: `3`)
- `--poll` — Queue worker poll interval (default: `10s`)
- `--session` — Target tmux session name (default: `main`)
