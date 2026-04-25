# tq — Task Queue

Job-queue based orchestration system for Claude Code. Combines a CLI/TUI with a Claude Code Plugin to manage multiple sessions via tmux.

Humans give instructions in natural language and monitor progress via TUI. The manager agent handles task breakdown, prompt tuning, and dispatching actions to worker sessions.

![tq demo](docs/tq.gif)

## Highlights

- **Job queue model** — actions are queued and processed asynchronously by workers, with configurable concurrency limits (default: 3 interactive sessions)
- **Manager agent** — one Claude Code session manages others; humans just talk to it
- **Delegates to Claude Code** — no custom workflow language; just runs `claude` in tmux, so skills, sub-agents, worktrees, and remote execution all work as-is
- **CLI is for agents** — JSON-only output, `--jq` flag, agent-oriented `--help`; humans don't need to learn the CLI
- **Schedule support** — cron-based action generation for recurring tasks

## Install

Prerequisites: [tmux](https://github.com/tmux/tmux) (required for dispatching interactive sessions)

```bash
# Homebrew (macOS / Linux)
brew install MH4GF/tap/tq

# Claude Code Plugin
claude plugin marketplace add MH4GF/tq
claude plugin install tq@tq-marketplace
```

Or download a binary from [GitHub Releases](https://github.com/MH4GF/tq/releases).

## Quick Start

```bash
# Register a project (sets working directory for tasks)
tq project create myapp ~/src/myapp

# Create a task under the project
tq task create "Implement feature X" --project 1

# Launch the TUI (starts the queue worker)
tq ui
```

Then open Claude Code in another terminal and talk to the manager agent:

```
/tq:manager
```

To create an action (spawns a new Claude Code session):

```
/tq:create-action Implement feature X and create a PR
```

The agent will tune the instruction, associate it with the right task, create the action, and the queue worker will automatically pick it up.

See [CLI Reference](docs/cli-reference.md) for the full command list.

## Configuration

The database lives at `~/.config/tq/tq.db` by default. Override with:

- `--db <path>` flag (highest precedence)
- `TQ_DB_PATH` environment variable

Useful for running multiple isolated queues (e.g. a separate DB for demos or testing).

## Data Model

```
project → task → action
```

- **project**: groups tasks, sets default working directory
- **task**: unit of work (status: open, done, archived)
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

                            cancel
              (from pending, running, or failed)
                              │
                              ▼
                        ┌───────────┐
                        │ cancelled │
                        └───────────┘

  * cancel/fail/reset only update the DB; tmux panes are not terminated
  * to restart a running action, run `tq action cancel` or `fail` first, then `reset`
  * done and cancelled are terminal; on_done spawns a new action from done only
```

### Worker Types

Controlled via `--meta` on `action create` / `schedule create`:

| mode | Description |
|------|-------------|
| `interactive` (default) | `claude` in tmux — fire-and-forget, worker reports via `tq action done` / `tq action fail` |
| `noninteractive` | `claude -p` — captures stdout, auto-completes |
| `remote` | Dispatched to remote worker |

Additional metadata keys:
- `claude_args` — Additional CLI arguments for claude (JSON array of strings, e.g. `["--permission-mode","plan","--worktree","--max-turns","5"]`)

## License

MIT
