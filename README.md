# tq — Task Queue

Job-queue based orchestration system for Claude Code. Combines a CLI/TUI with a Claude Code Plugin to manage multiple sessions via tmux.

Humans give instructions in natural language and monitor progress via TUI. The manager agent handles task breakdown, prompt tuning, and dispatching actions to worker sessions.

## Highlights

- **Job queue model** — actions are queued and processed asynchronously by workers, with configurable concurrency limits (default: 3 interactive sessions)
- **Manager agent** — one Claude Code session manages others; humans just talk to it
- **Delegates to Claude Code** — no custom workflow language; just runs `claude` in tmux, so skills, sub-agents, worktrees, and remote execution all work as-is
- **CLI is for agents** — JSON-only output, `--jq` flag, agent-oriented `--help`; humans don't need to learn the CLI
- **Schedule support** — cron-based action generation for recurring tasks

## Install

Prerequisites: [tmux](https://github.com/tmux/tmux) (required for dispatching interactive sessions)

```bash
# CLI
go install github.com/MH4GF/tq@latest

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

## Data Model

```
project → task → action
```

- **project**: groups tasks, sets default working directory
- **task**: unit of work (status: open, review, done, blocked, archived)
- **action**: dispatchable unit — an instruction that becomes a Claude Code session (status: pending, running, done, failed, cancelled)

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

## License

MIT
