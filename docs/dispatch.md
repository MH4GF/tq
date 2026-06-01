# Queue Worker Dispatch Model

The `tq ui` queue worker (`dispatch.RunWorker`) polls pending actions and dispatches them to mode-specific executors. This document describes the concurrency model, slot caps, and how `claude_session_id` is captured.

All local actions launch via `claude --bg`, registering with the Claude Code daemon supervisor so they appear in `claude agents` ([Agent View docs](https://code.claude.com/docs/en/agent-view#manage-multiple-agents-with-agent-view)). The `mode` value picks which slot pool the action consumes; the launch mechanic is the same for `interactive` and `noninteractive`. Only `remote` follows a different path (`claude --remote`).

## Concurrency model

```
dispatch loop (1 goroutine)
─────────────────────────
NextPending ─┬─ interactive    ──► claude --bg (admit if Interactive ≤ MaxI)
             ├─ noninteractive ──► claude --bg (admit if NonI ≤ MaxNI)
             └─ remote         ──► claude --remote (fire-and-forget)

reapBg / CheckSchedules / Heartbeat   ← always run each poll
```

`Running` is the live `COUNT(*)` of running actions of that mode **including the just-claimed action** (NextPending marks status=running before the admission check). The predicate is `running > Max` so `MaxInteractive=N` / `MaxNonInteractive=N` means "up to N concurrent" inclusive.

The dispatch loop processes one action per iteration. Each `claude --bg` invocation returns the moment the daemon accepts the session, so the loop is never blocked on a long-running worker. `reapBg`, `CheckSchedules`, and `UpdateWorkerHeartbeat` always run each poll iteration.

## Slot caps

`MaxInteractive` and `MaxNonInteractive` are **independent** slot counts checked against the live DB `running` count immediately after `NextPending` claims an action.

| Cap | Default | Purpose |
| --- | --- | --- |
| `MaxInteractive` | 3 | Cognitive-load cap on simultaneous user-facing sessions surfaced in `claude agents`. |
| `MaxNonInteractive` | 5 | Throughput pool for high-volume / scheduled work so batch fleets do not crowd out interactive sessions. |

A pending action that would exceed its cap is returned to `pending` via `DeferToPending(id, defaultDeferBackoff)`, which also stamps `dispatch_after = now + 30s`. `NextPending`'s WHERE clause then skips the deferred row for that window, letting the worker attempt other pending actions on the next poll.

Manual `tq action reset` calls `ResetToPending`, which also clears `dispatch_after` so the action is immediately eligible for `NextPending` again.

### Legacy `experimental_bg` compatibility shim

`experimental_bg` was the old name for the bg-launched mode. The `Migrate()` step rewrites pending rows and `settings.default_mode` from `experimental_bg` to `interactive`, but **running** rows are left untouched on purpose so an in-flight session is not flipped under the feet of its daemon. The interactive predicate therefore still recognizes legacy `experimental_bg` running rows and counts them toward `MaxInteractive`. The reaper picks them up by daemon_short, not by mode. Plan to remove the shim in a future release once any legacy running rows have terminated.

## `claude_session_id` capture

Local bg actions do **not** inherit `TQ_ACTION_ID` into the daemon-spawned claude process — the daemon claims a pre-spawned spare session, so the `SessionStart` hook silently no-ops for these actions. Instead, the bg reaper reads `~/.claude/jobs/<short>/state.json` each tick and back-fills `metadata.claude_session_id` from the daemon-recorded `sessionId` field the first time it appears. Cloud-executed actions still rely on the `SessionStart` hook because they retain `TQ_ACTION_ID` in their environment.

The session ID is used for `claude --resume` and for log investigation on failure. Capture is best-effort: a missing session ID does not fail the action.

## Cloud-executed actions are exempt from reaping

Actions whose `metadata.executor` is `"cloud"` are skipped by `reapBg` because they have no local daemon job dir for the reaper to probe — liveness is the responsibility of the cloud session that opened the action, which closes it via `tq action done|fail`.

`executor` is stamped:
- by `tq action create --status running` when invoked from a cloud session (`CLAUDE_CODE_REMOTE=true`)
- by the `SessionStart` hook (`tq internal claude-session-record`) when the session inherits a `TQ_ACTION_ID` and runs in cloud

`mode=remote` (tq's own RemoteWorker dispatch) terminates with `MarkDispatched`, so its actions reach `status=dispatched` and are not picked up by the reaper's `status=running` queries — the executor exemption is for `status=running` actions only.

## Action lifecycle

```
            ┌──────────┐
            │ pending  │◄────────────────┐
            └────┬─────┘ DeferToPending  │ (slot cap reached;
                 │ NextPending           │  dispatch_after = now+30s)
                 ▼                       │
            ┌──────────┐                 │
   ┌────────│ running  │─────────────────┘
   │        └────┬─────┘
   │             │ worker.Execute
   │             │   bg launch returns immediately with daemon_short
   │             │   remote ends in MarkDispatched
   │       ┌─────┴─────┐
   ▼       ▼           ▼
┌──────┐ ┌──────┐ ┌──────────────┐
│ done │ │failed│ │ failed       │ ← reapBg (missing job dir grace)
└──────┘ └──────┘ └──────────────┘
```

`reapBg` polls `~/.claude/jobs/<short>/state.json` each tick: `state=done` → `BulkMarkDone(output.result)`, `state=failed` → `BulkMarkFailed(detail || output.result)`. Non-terminal states (`working`, `blocked`, …) leave the action `running`; `blocked` in particular means the bg session is awaiting reply via `claude agents`, not a failure. If the job dir disappears entirely (`os.IsNotExist`) and the action has been running longer than `BgMissingJobGrace` (default 30s), the action is failed.

## Shutdown

`RunWorker` returns when its context is cancelled. The bg launch path is fire-and-forget once the daemon accepts the session, so there are no in-flight executor goroutines to drain at shutdown.
