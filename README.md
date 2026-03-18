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

# Create an action linked to a prompt template
tq action create review-pr --task 1 --title "Review PR #42"

# Dispatch the next pending action
tq dispatch

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
- **action**: dispatchable item linked to a prompt template (status: pending, running, done, failed, cancelled)

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

| auto | interactive | Execution |
|------|------------|-----------|
| true | false | `claude -p` — captures stdout, auto-completes |
| true | true | `claude --worktree --tmux` — fire-and-forget, worker reports via `tq action done` |
| false | * | Not dispatched — waits for human |

### Prompts

Defined as frontmatter-annotated markdown in `~/.config/tq/prompts/`:

```markdown
---
description: Generic implementation task
auto: true
interactive: true
max_retries: 0
on_done: review
---
{{.Action.Meta.instruction}}

When done: tq action done {{.Action.ID}} "<summary of what was done>"
```

Available template variables: `{{.Task.ID}}`, `{{.Task.Title}}`, `{{.Task.URL}}`, `{{.Project.Name}}`, `{{.Project.WorkDir}}`, `{{.Action.ID}}`, `{{.Action.Meta.<key>}}`

> **Result format**: The second argument to `tq action done` is a free-form string (not necessarily JSON). If the prompt has `on_done` configured, the result is passed to the follow-up action as `{{.Action.Meta.predecessor_result}}`. Include any information the next action needs — for example, a PR URL if the action created one.

## CLI Reference

| Command | Description |
|---------|-------------|
| `tq project create <NAME> <WORK_DIR>` | Register a project |
| `tq project list` | List projects (JSON) |
| `tq project delete <ID>` | Delete a project |
| `tq task create <TITLE> --project <ID>` | Create a task |
| `tq task list` | List tasks with nested actions (JSON) |
| `tq task update <ID> --status <STATUS>` | Update task status |
| `tq action create <PROMPT> --task <ID> --title <TITLE>` | Create an action |
| `tq action list` | List actions (JSON) |
| `tq action done <ID> [RESULT]` | Mark action as done |
| `tq action reset <ID>` | Reset action to pending |
| `tq dispatch <ACTION_ID>` | Dispatch a specific action |
| `tq dispatch` | Dispatch next pending action |
| `tq schedule create <PROMPT> --task <ID> --title <TITLE> --cron <EXPR>` | Create a schedule |
| `tq schedule list` | List schedules |
| `tq prompt list` | List available prompt templates |
| `tq ui` | Launch TUI with queue worker |

## License

MIT
