# tq — Task Queue

Job-queue based orchestration system for Claude Code. Combines a CLI/TUI with a Claude Code Plugin to manage multiple Claude Code sessions.

Humans give instructions in natural language and monitor progress via TUI. The manager agent handles task breakdown, prompt tuning, and dispatching actions to worker sessions.

All local actions are dispatched as Agent View background sessions via `claude --bg`, so they appear in `claude agents` and consume your regular Claude Code subscription usage (no extra Agent SDK billing — see [Agent View docs](https://code.claude.com/docs/en/agent-view#manage-multiple-agents-with-agent-view)).

![tq demo](docs/tq.gif)

## Highlights

- **Job queue model** — actions are queued and processed asynchronously by workers, with independent concurrency caps for the interactive slot pool (default: 3) and the noninteractive slot pool (default: 5)
- **Manager agent** — one Claude Code session manages others; humans just talk to it
- **Delegates to Claude Code** — no custom workflow language; every local action runs as a `claude --bg` background session, so skills, sub-agents, worktrees, and remote execution all work as-is
- **CLI is for agents** — JSON-only output, `--jq` flag, agent-oriented `--help`; humans don't need to learn the CLI
- **Schedule support** — cron-based action generation for recurring tasks

## Install

Prerequisites: [Claude Code v2.1.139+](https://code.claude.com/docs/en/agent-view) with agent view enabled (needed by `claude --bg`)

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
- **Queue Worker** — every local action launches as a `claude --bg` Agent View background session; the dispatch loop is never blocked because `claude --bg` returns the moment the daemon accepts the session. Lifecycle is driven by polling `~/.claude/jobs/<short>/state.json`. The two slot pools (`MaxInteractive`, `MaxNonInteractive`) are independent caps on how many sessions can run concurrently per mode.
- **AI is hands only** — Go handles orchestration; AI executes as a `claude` worker
- **TUI is observation-first** — humans monitor via TUI; mutations are issued through the CLI, with a few in-TUI convenience shortcuts (`f` toggle focus, `r` resume, `d` dispatch a pending action) that delegate to the same operations

### Action State Machine

```
         dispatch        worker claim
 pending ────────► running ───────────► dispatched

  Terminal transitions (each reachable from the states noted):

  done   (from pending, running, or dispatched)
                              │
                              ▼
                        ┌────────┐
                        │  done  │
                        └────────┘

  fail   (from pending, running, or dispatched)
                              │
                              ▼
              reset       ┌────────┐
    pending ◄──────────── │ failed │
                          └────────┘

  cancel (from pending, running, dispatched, or failed)
                              │
                              ▼
                        ┌───────────┐
                        │ cancelled │
                        └───────────┘

  * done can be issued from any non-terminal state (pending, running, or dispatched)
  * fail can be issued from any non-terminal state (pending, running, or dispatched)
  * cancel can be issued from pending, running, dispatched, or failed (terminal: cancelled)
  * reset returns failed or cancelled actions to pending; running and dispatched
    must be cancelled or failed first (reset is rejected to avoid spawning a duplicate worker)
  * cancel/fail/reset only update the DB; tmux panes are not terminated
  * done, failed, and cancelled are terminal; on_done spawns a new action from done only
  * `tq action resume <id>` spawns a new action that resumes the claude session of any terminal action whose metadata captured `claude_session_id` (see `docs/cli-reference.md`)
  * completion dependencies (`--blocked-by-action`/`--blocked-by-task`) hold a
    pending action until every blocker reaches a successful terminal state
    (action=done, task=done/archived); a failed/cancelled blocker holds it
    forever (rescue via `/tq:dep-triage`, or delete the blocker — including
    `tq project delete --cascade` — which purges the edge). `tq action
    dispatch` is the manual bypass; the queue worker respects the gate (see
    `docs/cli-reference.md`)
```

### Worker Types

Controlled via `--meta` on `action create` / `schedule create`. Any value outside this set is rejected — pass Claude permission-mode (`auto`, `plan`, `acceptEdits`, …) via `claude_args` instead.

| mode | Description |
|------|-------------|
| `interactive` (default) | `claude --bg` background session that consumes the interactive slot pool (`MaxInteractive`, default 3). Lifecycle tracked by polling `~/.claude/jobs/<short>/state.json`. |
| `noninteractive` | `claude --bg` background session that consumes the noninteractive slot pool (`MaxNonInteractive`, default 5). Same launch path as `interactive`; the distinct pool exists to keep many short batch sessions from starving long-running interactive ones. |
| `remote` | `claude --remote` for cloud execution. The fire-and-forget cloud session reports back via `tq action done` / `tq action fail`. |

Additional metadata keys:
- `claude_args` — Additional CLI arguments for claude (JSON array of strings, e.g. `["--permission-mode","plan","--worktree","--max-turns","5"]`)

## Testing

Unit tests live alongside source files (`*_test.go`). End-to-end CLI scenarios are written as [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) files under `e2e/testdata/script/*.txtar`. Each scenario runs against an isolated `TQ_DB_URL` and `HOME`, so they can run in parallel without interference. Run all tests with `go test ./...`, or just the E2E suite with `go test ./e2e`. To add a scenario, drop a new `.txtar` file describing the CLI invocations and expected stdout/stderr.

The same e2e suite can be replayed against a libsql endpoint to verify driver compatibility. With a sqld container (or Turso DB) reachable, set `TQ_DB_URL=libsql://...` and run `go test -tags libsql_e2e ./e2e/ -run TestLibsqlE2E`. The libsql variant resets the schema between scenarios since they share one DB. CI runs this against a sqld service container on every PR.

## License

MIT
