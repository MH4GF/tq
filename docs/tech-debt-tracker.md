# Tech Debt Tracker

Current violation inventory for golden rules that have non-zero counts. Updated by running `go test ./internal/goldenrules/ -v` and recording the output.

## Rule 11 — Raw SQL in upper layers (30 violations)

Ceiling: 30 (as of 2026-04-12). Lower as violations are fixed.

### cmd/reset_test.go (1)

- `cmd/reset_test.go:78` — `d.Exec("UPDATE actions SET status = 'open' WHERE id = 1")`

### dispatch/queue_worker_test.go (17)

- `dispatch/queue_worker_test.go:76` — `d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 2")`
- `dispatch/queue_worker_test.go:206` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:233` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:257` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now') WHERE id = 1")`
- `dispatch/queue_worker_test.go:281` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:305` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1' WHERE id = 1")`
- `dispatch/queue_worker_test.go:370` — `d.Exec("UPDATE actions SET session_id = 'session-1' WHERE id = 3")`
- `dispatch/queue_worker_test.go:411` — `d.Exec("UPDATE actions SET session_id = 'work', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:439` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-25 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:463` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:514` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:542` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:570` — `d.Exec("UPDATE actions SET session_id = 'main', tmux_pane = 'tq-action-1', started_at = datetime('now', '-5 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:595` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-25 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:619` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-25 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:643` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-25 minutes') WHERE id = 1")`
- `dispatch/queue_worker_test.go:667` — `d.Exec("UPDATE actions SET started_at = datetime('now', '-25 minutes') WHERE id = 1")`

### dispatch/schedule_test.go (7)

- `dispatch/schedule_test.go:20` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")`
- `dispatch/schedule_test.go:53` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:00:00' WHERE id = 1")`
- `dispatch/schedule_test.go:72` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")`
- `dispatch/schedule_test.go:94` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = ?", id)`
- `dispatch/schedule_test.go:114` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")`
- `dispatch/schedule_test.go:133` — `d.Exec("UPDATE schedules SET created_at = '2026-03-12 09:58:00' WHERE id = 1")`
- `dispatch/schedule_test.go:161` — `d.Exec("UPDATE schedules SET created_at = '2026-03-01 00:00:00', last_run_at = '2026-03-12 09:00:00' WHERE id = ?", id)`

### tui/tasks_test.go (5)

- `tui/tasks_test.go:184` — `d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE title ='old-action'")`
- `tui/tasks_test.go:185` — `d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', ..."))`
- `tui/tasks_test.go:230` — `d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE title ='old-action'")`
- `tui/tasks_test.go:251` — `d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00', completed_at = '2025-01-01 00:00:00' WHERE title ='old-action'")`
- `tui/tasks_test.go:252` — `d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', ..."))`

## Remediation approach

All 30 violations bypass `db.Store` to set timestamps or session state directly via raw SQL in test files. The fix is to add narrow test-seam methods to `db.Store` (e.g., `SetActionStartedAt`, `SetScheduleCreatedAt`) and route tests through the interface. This will be handled by a separate GC action.
