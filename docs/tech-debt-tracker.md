# Tech Debt Tracker

Current violation inventory for golden rules that have non-zero counts. Updated by running `go test ./internal/goldenrules/ -v` and recording the output.

No active violations.

## Rule 11 — Raw SQL in upper layers (resolved)

All 30 violations resolved by adding test-seam methods to `db.Store` (`SetActionSessionInfoForTest`, `SetScheduleTimestampsForTest`, `SetActionTimestampsForTest`, `SetTaskTimestampsForTest`, `SetActionStatusForTest`) and routing test setup through the interface. Ceiling lowered to 0.
