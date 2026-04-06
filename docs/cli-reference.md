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
| `tq task list` | List tasks with nested actions (JSON) |
| `tq task get <ID>` | Get a task by ID (JSON) |
| `tq task update <ID>` | Update a task |

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
- `--status` — Filter by status (`open`, `review`, `done`, `blocked`, `archived`)
- `--jq` — Filter JSON output (fields: `id`, `project_id`, `title`, `metadata`, `status`, `work_dir`, `created_at`, `updated_at`, `actions`)
- `--limit` — Limit number of results

### `tq task update`

```
tq task update <ID> [--status <STATUS>] [--project <ID>] [--work-dir <PATH>] [--meta <JSON>]
```

At least one flag is required.

- `--status` — New status (`open`, `review`, `done`, `blocked`, `archived`)
- `--project` — Project ID
- `--work-dir` — Working directory
- `--meta` — JSON metadata to merge

## action

| Command | Description |
|---------|-------------|
| `tq action create <INSTRUCTION> --task <ID> --title <TITLE>` | Create an action |
| `tq action list` | List actions (JSON) |
| `tq action get <ID>` | Get an action by ID (JSON) |
| `tq action update <ID>` | Update an action |
| `tq action done <ID> [RESULT]` | Mark action as done |
| `tq action cancel <ID> [REASON]` | Cancel an action |
| `tq action attach <ID>` | Attach to a running action's tmux window |
| `tq action reset <ID>` | Reset action to pending |
| `tq action dispatch <ID>` | Dispatch immediately (skip queue) |

### `tq action create`

```
tq action create <INSTRUCTION> --task <ID> --title <TITLE> [--meta <JSON>] [--status <STATUS>]
```

- `--task` — Task ID (**required**)
- `--title` — Action title (**required**, max 100 chars)
- `--meta` — JSON metadata for dispatch control:
  - `mode` — `"interactive"` (default), `"noninteractive"`, `"remote"`
  - `permission_mode` — Claude permission mode (e.g. `"plan"`, `"auto"`)
  - `worktree` — Run in git worktree (`true`/`false`)
- `--status` — Initial status (default: `pending`)

### `tq action list`

```
tq action list [--task <ID>] [--status <STATUS>] [--jq <EXPR>] [--limit <N>]
```

- `--task` — Filter by task ID
- `--status` — Filter by status (`pending`, `running`, `done`, `failed`, `cancelled`)
- `--jq` — Filter JSON output (fields: `id`, `title`, `task_id`, `metadata`, `status`, `result`, `session_id`, `started_at`, `completed_at`, `created_at`)
- `--limit` — Limit number of results

### `tq action done`

```
tq action done <ACTION_ID> [RESULT]
```

RESULT is free-form text. Recommended structure:

- **outcome** — What changed (concrete deliverables)
- **decisions** — What was decided and why
- **artifacts** — PR numbers, file paths, commit SHAs, URLs
- **remaining** — Unfinished work, known issues, follow-up needed

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
tq event list [--entity <TYPE>] [--id <ID>] [--limit <N>]
```

- `--entity` — Filter by entity type (`action`, `task`, `project`, `schedule`)
- `--id` — Filter by entity ID (requires `--entity`)
- `--limit` — Number of recent events to show (default: 50)

## search

```
tq search <KEYWORD>
```

Full-text search across task titles, task metadata, action titles, action results, and action metadata. Output is JSON.

## ui

```
tq ui [--max-interactive <N>] [--poll <DURATION>] [--session <NAME>]
```

- `--max-interactive` — Maximum concurrent interactive sessions (default: `3`)
- `--poll` — Queue worker poll interval (default: `10s`)
- `--session` — Target tmux session name (default: `main`)
