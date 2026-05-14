# Queue Worker Dispatch Model

The `tq ui` queue worker (`dispatch.RunWorker`) polls pending actions and dispatches them to mode-specific executors. This document describes the concurrency model, slot caps, and how `claude_session_id` is captured.

## Concurrency model

```
dispatch loop (1 goroutine)         executor goroutines (N + M in flight)
─────────────────────────           ───────────────────────────────────────
NextPending ─┬─ interactive     ──► sync worker.Execute (tmux: returns fast)
             │   admit if Interactive+Bg ≤ MaxI
             │
             ├─ noninteractive  ──► ┌─ go worker.Execute (claude -p, long-running)
             │   admit if Running   │   └ MarkDone / MarkFailed
             │     ≤ MaxNI          │
             │
             ├─ remote          ──► sync worker.Execute (returns fast)
             │
             └─ experimental_bg ──► sync worker.Execute (`claude --bg`, returns fast)
                 admit if Interactive+Bg ≤ MaxI    lifecycle polled by reapBg

reapStaleActions / reapBg / CheckSchedules / Heartbeat   ← always run each poll
```

`Running` is the live `COUNT(*)` of running actions of that mode **including the just-claimed action** (NextPending marks status=running before the admission check). The predicate is `running > Max` so `MaxInteractive=N` / `MaxNonInteractive=N` means "up to N concurrent" inclusive. `experimental_bg` shares the `MaxInteractive` slot pool with `interactive` because both surface in `claude agents` as user-interactive sessions; `CountRunningInteractiveOrBg` is the joint counter used by `BeforeInteractive` and `BeforeBg`.

The dispatch loop processes one action per iteration but **does not block on noninteractive execution** — `claude -p` runs in a per-action goroutine so the loop can immediately move on to dispatch the next action. `reapStaleActions`, `CheckSchedules`, and `UpdateWorkerHeartbeat` always run each poll iteration regardless of in-flight executor goroutines.

## Slot caps

`MaxInteractive` and `MaxNonInteractive` are **independent** slot counts checked against the live DB `running` count immediately after `NextPending` claims an action:

| Cap | Default | Purpose |
| --- | --- | --- |
| `MaxInteractive` | 3 | Cognitive-load cap on simultaneous user-facing sessions (interactive tmux + experimental_bg via `claude agents` share this pool) |
| `MaxNonInteractive` | 5 | OS resource cap (each `claude -p` process plus its MCP servers / hooks ≈ 200-400MB) |

A pending action that would exceed its cap is reset to `pending` (`ResetToPending`) and retried on the next poll. There is no in-memory semaphore — the DB row count is the source of truth, so the cap is correct even if the worker restarts mid-flight.

`MaxNonInteractive` is not a cognitive-load limit; it exists only to bound memory consumption when many noninteractive actions are queued. Override via `--max-noninteractive` based on available RAM.

## `claude_session_id` capture

The Claude Code `SessionStart` hook (`.claude-plugins/tq/`) is the sole writer of `metadata.claude_session_id`. When a tq-dispatched claude session starts, the hook invokes `tq internal claude-session-record` with `TQ_ACTION_ID` set, which merges the session ID into the action's metadata.

**Note**: `experimental_bg` mode does not auto-populate `claude_session_id`. The daemon's claimed-spare model means the bg session is pre-spawned and does not inherit `TQ_ACTION_ID` from the launcher, so the hook silently no-ops. The daemon does record its own `sessionId` in `~/.claude/jobs/<short>/state.json`, which the reaper can back-fill into the action if needed in a future iteration.

The session ID is used for `claude --resume` and for log investigation on failure. Capture is best-effort: a missing session ID does not fail the action. The reaper still probes `~/.claude/projects` session log mtime as a liveness signal (`ClaudeSessionLogChecker.IsClaudeSessionActive`), but no longer writes the session ID itself.

## Cloud-executed actions are exempt from reaping

Actions whose `metadata.executor` is `"cloud"` are skipped by `reapStaleActions` in both interactive and noninteractive loops. Such actions run in Claude Code on the web (including Cloud Routines) and have no local tmux window or session log for the reaper to probe — liveness is the responsibility of the cloud session that opened the action, which closes it via `tq action done|fail`.

`executor` is stamped:
- by `tq action create --status running` when invoked from a cloud session (`CLAUDE_CODE_REMOTE=true`)
- by the `SessionStart` hook (`tq internal claude-session-record`) when the session inherits a `TQ_ACTION_ID` and runs in cloud

`mode=remote` (tq's own RemoteWorker dispatch) terminates with `MarkDispatched`, so its actions reach `status=dispatched` and are not picked up by the reaper's `status=running` queries — the executor exemption is for `status=running` actions only.

## Action lifecycle

```
            ┌──────────┐
            │ pending  │◄────────────────┐
            └────┬─────┘ ResetToPending  │ (slot cap reached)
                 │ NextPending           │
                 ▼                       │
            ┌──────────┐                 │
   ┌────────│ running  │─────────────────┘
   │        └────┬─────┘
   │             │ worker.Execute
   │             │   interactive / remote: synchronous
   │             │   noninteractive: async goroutine (MarkDone in goroutine)
   │       ┌─────┴─────┐
   ▼       ▼           ▼
┌──────┐ ┌──────┐ ┌──────────────┐
│ done │ │failed│ │ failed       │ ← reapStaleActions / hard timeout
└──────┘ └──────┘ │ (stale/early)│
                  └──────────────┘
```

For `experimental_bg`, the worker returns the daemon short id immediately (action stays `running`) and `reapBg` polls `~/.claude/jobs/<short>/state.json` each tick: `state=done` → `BulkMarkDone(output.result)`, `state=failed` → `BulkMarkFailed(detail || output.result)`. Non-terminal states (`working`, `blocked`, …) leave the action `running`; `blocked` in particular means the bg session is awaiting reply via `claude agents`, not a failure.

## Shutdown

`RunWorker` returns when its context is cancelled. Before returning it calls `wg.Wait()` to drain in-flight noninteractive goroutines so their `MarkDone` / `MarkFailed` writes complete. Context cancellation propagates to each `worker.Execute`, which terminates the underlying `claude -p` subprocess via `exec.CommandContext`.
