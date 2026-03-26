# tq — Task Queue

AI-powered task queue backed by SQLite. Dispatch work to Claude Code workers via tmux.

## Features

- **SQLite as single source of truth** — all state lives in `~/.config/tq/tq.db`
- **Queue Worker** — automatically picks up pending actions and dispatches them to Claude Code sessions
- **AI is hands only** — orchestration is in Go; AI just executes via `claude` CLI
- **TUI is read-only** — humans monitor via TUI (`tq ui`); mutations go through CLI commands

## Install

```bash
go install github.com/MH4GF/tq@latest
```

Or download a binary from [GitHub Releases](https://github.com/MH4GF/tq/releases).

## Quick Start

```bash
# Register a project (sets working directory for tasks)
tq project create myapp ~/src/myapp

# Create a task under the project
tq task create "Implement feature X" --project 1

# Create an action with an instruction
tq action create "/github-pr review this" --task 1 --title "Review PR #42"

# Dispatch a pending action by ID
tq action dispatch 1

# Launch the TUI (includes queue worker)
tq ui
```

## Architecture

### Data Model

```
project → task → action
```

- **project**: groups tasks, sets default working directory
- **task**: unit of work (status: open, review, done, blocked, archived)
- **action**: dispatchable unit of work with an instruction (status: pending, running, done, failed, cancelled)

### Design Principles

- **SQLite is SSOT** — the database is the canonical state store
- **Queue Worker** — processes one action at a time, keeping context fresh
- **AI is hands only** — Go handles orchestration; AI executes as a `claude` worker
- **TUI is read-only** — humans observe via TUI; operations are issued through CLI

### Action State Machine

```
                          dispatch/claim
                ┌─────────────────────────────┐
                │                             ▼
          ┌───────────┐                ┌───────────┐
          │  pending   │                │  running   │
          └───────────┘                └─────┬─────┘
                ▲                        │         │
                │                 success│         │fail
                │                        ▼         ▼
                │                  ┌────────┐  ┌────────┐
                │                  │  done   │  │ failed │
                │                  └────┬───┘  └───┬────┘
                │                       │          │
                │              on_done  │          │ reset
                │         (new action)  │          │
                └───────────────────────┘          │
                └──────────────────────────────────┘

  * running → pending: reset command (kills tmux pane)
  * done is terminal, but on_done can spawn a new action
```

### Worker Types

Controlled via `--meta` on `action create` / `schedule create`:

| mode | Description |
|------|-------------|
| `interactive` (default) | `claude` in tmux — fire-and-forget, worker reports via `tq action done` |
| `noninteractive` | `claude -p` — captures stdout, auto-completes |
| `remote` | Dispatched to remote worker |

Additional metadata keys:
- `permission_mode` — Claude permission mode (e.g. `"plan"`, `"auto"`)
- `worktree` — Run in a git worktree for isolation (`true`/`false`)

## CLI Reference

| Command | Description |
|---------|-------------|
| `tq project create <NAME> <WORK_DIR>` | Register a project |
| `tq project list` | List projects (JSON) |
| `tq project update <ID>` | Update a project |
| `tq project delete <ID>` | Delete a project |
| `tq task create <TITLE> --project <ID>` | Create a task |
| `tq task list` | List tasks with nested actions (JSON) |
| `tq task get <ID>` | Get a task by ID (JSON) |
| `tq task update <ID> --status <STATUS>` | Update task status |
| `tq action create <INSTRUCTION> --task <ID> --title <TITLE>` | Create an action |
| `tq action list` | List actions (JSON) |
| `tq action get <ID>` | Get an action by ID (JSON) |
| `tq action update <ID>` | Update an action |
| `tq action done <ID> [RESULT]` | Mark action as done |
| `tq action cancel <ID>` | Cancel an action |
| `tq action attach <ID>` | Attach to a running action's tmux window |
| `tq action reset <ID>` | Reset action to pending |
| `tq action dispatch <ACTION_ID>` | Dispatch an action by ID |
| `tq schedule create --instruction <TEXT> --task <ID> --cron <EXPR> [--meta <JSON>]` | Create a schedule |
| `tq schedule list` | List schedules (JSON) |
| `tq schedule update <ID>` | Update a schedule |
| `tq schedule delete <ID>` | Delete a schedule |
| `tq schedule enable <ID>` | Enable a schedule |
| `tq schedule disable <ID>` | Disable a schedule |
| `tq event list` | List events |
| `tq search <KEYWORD>` | Search tasks and actions |
| `tq ui` | Launch TUI with queue worker |

## License

MIT
