# Queue Worker Dispatch Model

The `tq ui` queue worker (`dispatch.RunWorker`) polls pending actions and dispatches them to mode-specific executors. This document describes the concurrency model, slot caps, and `claude_session_id` discovery paths.

## Concurrency model

```
dispatch loop (1 goroutine)         executor goroutines (N + M in flight)
─────────────────────────           ───────────────────────────────────────
NextPending ─┬─ interactive     ──► sync worker.Execute (tmux: returns fast)
             │   admit if Running ≤ MaxI
             │
             ├─ noninteractive  ──► ┌─ go worker.Execute (claude -p, long-running)
             │   admit if Running   │   ├ saveClaudeSessionID
             │     ≤ MaxNI          │   └ MarkDone / markActionFailed
             │
             └─ remote          ──► sync worker.Execute (returns fast)

reapStaleActions / CheckSchedules / Heartbeat   ← always run each poll
```

`Running` is the live `COUNT(*)` of running actions of that mode **including the just-claimed action** (NextPending marks status=running before the admission check). The predicate is `running > Max` so `MaxInteractive=N` / `MaxNonInteractive=N` means "up to N concurrent" inclusive.

The dispatch loop processes one action per iteration but **does not block on noninteractive execution** — `claude -p` runs in a per-action goroutine so the loop can immediately move on to dispatch the next action. `reapStaleActions`, `CheckSchedules`, and `UpdateWorkerHeartbeat` always run each poll iteration regardless of in-flight executor goroutines.

## Slot caps

`MaxInteractive` and `MaxNonInteractive` are **independent** slot counts checked against the live DB `running` count immediately after `NextPending` claims an action:

| Cap | Default | Purpose |
| --- | --- | --- |
| `MaxInteractive` | 3 | Cognitive-load cap on simultaneous interactive (tmux) sessions a human is reviewing |
| `MaxNonInteractive` | 5 | OS resource cap (each `claude -p` process plus its MCP servers / hooks ≈ 200-400MB) |

A pending action that would exceed its cap is reset to `pending` (`ResetToPending`) and retried on the next poll. There is no in-memory semaphore — the DB row count is the source of truth, so the cap is correct even if the worker restarts mid-flight.

`MaxNonInteractive` is not a cognitive-load limit; it exists only to bound memory consumption when many noninteractive actions are queued. Override via `--max-noninteractive` based on available RAM.

## `claude_session_id` discovery paths

| Mode | Phase | How session ID is captured |
| --- | --- | --- |
| Interactive | Running, after `StaleGracePeriod` (30 s) | `reapCheckClaudeSessionLog` runs every poll, saves to `metadata.claude_session_id` while validating heartbeat |
| Noninteractive | Running, before `staleThreshold` (~20 min) | Not captured (`reapStaleActions` only probes after the threshold) |
| Noninteractive | Running, at/after `staleThreshold` | `reapCheckClaudeSessionLog` saves to `metadata.claude_session_id` |
| Noninteractive | Immediately after `worker.Execute` returns | `saveClaudeSessionID` runs inside the executor goroutine using `postExecutionFreshness` (5 min) |

The session ID is used for `claude --resume` and for log investigation on failure. Capture is best-effort: a missing session ID does not fail the action.

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

## Shutdown

`RunWorker` returns when its context is cancelled. Before returning it calls `wg.Wait()` to drain in-flight noninteractive goroutines so their `MarkDone` / `markActionFailed` writes complete. Context cancellation propagates to each `worker.Execute`, which terminates the underlying `claude -p` subprocess via `exec.CommandContext`.
