# tq — Task Queue

Job-queue based orchestration system for Claude Code. Combines a CLI/TUI with a Claude Code Plugin to manage multiple sessions via tmux.

Humans give instructions in natural language and monitor progress via TUI. The manager agent handles task breakdown, prompt tuning, and dispatching actions to worker sessions.

![tq demo](docs/tq.gif)

## Highlights

- **Job queue model** — actions are queued and processed asynchronously by workers, with independent concurrency caps for interactive (default: 3) and noninteractive (default: 5) execution
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

- `--db <path-or-url>` flag (highest precedence)
- `TQ_DB_URL` environment variable

Both accept either a local sqlite file path or a libsql URL
(`libsql://...?authToken=...`), so the same binary can talk to a remote
[Turso](https://turso.tech) database (or any libsql-compatible endpoint
such as a self-hosted [sqld](https://github.com/tursodatabase/libsql)).
Verified against Turso. Embed the auth token in the URL query string;
no separate env var is read.

Useful for running multiple isolated queues (e.g. a separate DB for demos or testing).

## Data Model

```
project → task → action
```

- **project**: groups tasks, sets default working directory
- **task**: unit of work (status: open, done, archived)
- **action**: dispatchable unit of work with an instruction (status: pending, running, dispatched, done, failed, cancelled)

### Design Principles

- **SQLite is SSOT** — the database is the canonical state store
- **Queue Worker** — interactive actions dispatch into tmux (one at a time per slot); noninteractive actions run concurrently in goroutines so a long-running `claude -p` does not block the dispatch loop
- **AI is hands only** — Go handles orchestration; AI executes as a `claude` worker
- **TUI is read-only** — humans observe via TUI; operations are issued through CLI

### Action State Machine

```
         dispatch        worker claim       worker report
 pending ────────► running ───────────► dispatched ─────────► done
    ▲                                       │
    │                                       │ fail
    │                                       ▼
    │              reset                ┌────────┐
    └────────────────────────────────── │ failed │
                                        └────────┘

                            cancel
              (from pending, running, dispatched, or failed)
                              │
                              ▼
                        ┌───────────┐
                        │ cancelled │
                        └───────────┘

  * cancel can be issued from pending, running, dispatched, or failed (terminal: cancelled)
  * fail can be issued from any non-terminal state (pending, running, or dispatched)
  * reset returns failed or cancelled actions to pending; running and dispatched
    must be cancelled or failed first (reset is rejected to avoid spawning a duplicate worker)
  * cancel/fail/reset only update the DB; tmux panes are not terminated
  * done, failed, and cancelled are terminal; on_done spawns a new action from done only
  * `tq action resume <id>` spawns a new action that resumes the claude session of any terminal action whose metadata captured `claude_session_id` (see `docs/cli-reference.md`)
```

### Worker Types

Controlled via `--meta` on `action create` / `schedule create`. Any value outside this set is rejected — pass Claude permission-mode (`auto`, `plan`, `acceptEdits`, …) via `claude_args` instead.

| mode | Description |
|------|-------------|
| `interactive` (default) | `claude` in tmux — fire-and-forget, worker reports via `tq action done` / `tq action fail`. |
| `noninteractive` | `claude -p` — captures stdout, auto-completes. Heartbeat-aware: kept alive past the 600s minimum while the session log mtime stays fresh (≤120s old), up to a 60-minute hard cap |
| `remote` | Dispatched to remote worker |

Additional metadata keys:
- `claude_args` — Additional CLI arguments for claude (JSON array of strings, e.g. `["--permission-mode","plan","--worktree","--max-turns","5"]`)

## Testing

Unit tests live alongside source files (`*_test.go`). End-to-end CLI scenarios are written as [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) files under `e2e/testdata/script/*.txtar`. Each scenario runs against an isolated `TQ_DB_URL`, `HOME`, and `TMUX_TMPDIR`, so they can run in parallel without interference. Run all tests with `go test ./...`, or just the E2E suite with `go test ./e2e`. To add a scenario, drop a new `.txtar` file describing the CLI invocations and expected stdout/stderr; for tmux dispatch scenarios, embed a `claude` stub via `-- file --` sections and synchronize with `tmux wait-for` to keep the test deterministic.

The same e2e suite can be replayed against a libsql endpoint to verify driver compatibility. With a sqld container (or Turso DB) reachable, set `TQ_DB_URL=libsql://...` and run `go test -tags libsql_e2e ./e2e/ -run TestLibsqlE2E`. The libsql variant resets the schema between scenarios since they share one DB. CI runs this against a sqld service container on every PR.

## License

MIT
