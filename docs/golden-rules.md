# Golden Rules

Verifiable architectural constraints for the tq codebase.

This file is the single source of truth for agent-driven "tech-debt garbage collection" — the practice of encoding architectural invariants as mechanical rules, then running background agents to detect and auto-fix deviations. See the pattern's origin: <https://alexlavaee.me/blog/openai-agent-first-codebase-learnings/> (pattern #5).

**Rules must be verifiable constraints, not taste preferences.** Every rule below has a `Verify` column with a concrete command, lint name, or detection approach. Rules that cannot be verified do not belong here.

Classification:

- **[enforced]** — already enforced mechanically by lint / compile / depguard. Deviations fail CI.
- **[mechanical]** — detectable by grep / ast-grep / a trivial script. Not yet wired into CI.
- **[agent]** — requires LLM judgment. Checked by the periodic GC action (Phase 2 deliverable).

Current status totals are captured after each rule as `current violations: N`. A non-zero count is **not** a bug in the rule — it is tech debt to be tracked in `docs/tech-debt-tracker.md` (Phase 2 deliverable) and burned down by GC action PRs.

---

## Rules

### Layer boundaries

**Rule 1 [enforced] — Layer dependency direction is `db ← dispatch/tui ← cmd ← main`.**

- Why: Keeps the foundation (db) free of orchestration concerns and lets the upper layers be swapped or tested without pulling in SQL.
- Verify: `.golangci.yml` depguard rule `deny-upward-from-db` rejects imports from `db/` into `cmd`, `dispatch`, `tui`. Run `golangci-lint run ./...`.
- Current violations: 0.

**Rule 2 [enforced] — `testutil/` MUST NOT import `cmd`, `dispatch`, or `tui`.**

- Why: `testutil` is a foundational helper shared by all layers' tests; importing upward creates cycles and couples test helpers to orchestration code.
- Verify: depguard `deny-upward-from-testutil`. Run `golangci-lint run ./...`.
- Current violations: 0.

**Rule 3 [enforced] — `dispatch/` and `tui/` MUST NOT import each other.**

- Why: Both are service-layer peers. Cross-imports would collapse the layered architecture into a mesh.
- Verify: depguard `deny-upward-from-dispatch` and `deny-upward-from-tui`. Run `golangci-lint run ./...`.
- Current violations: 0.

### DB access abstraction

**Rule 4 [mechanical] — Upper layers depend on the `db.Store` interface, not the concrete `*db.DB` type.**

- Why: The `Store` interface (composed of `CommandWriter` + `QueryReader`, defined in `db/interfaces.go`) is the contract upper layers rely on. Depending on `*db.DB` directly leaks implementation details and blocks substitution in tests.
- Verify: `grep -rn '\*db\.DB' cmd/ dispatch/ tui/` must return zero matches.
- Current violations: 0.

### Test strategy

**Rule 5 [mechanical] — Tests use `testutil.NewTestDB(t)` (in-memory SQLite) rather than calling `db.Open` directly.**

- Why: `testutil.NewTestDB` guarantees migration is applied and cleanup is registered with `t.Cleanup`. Hand-rolled `db.Open(":memory:")` calls diverge from this and silently skip migrations.
- Verify: `grep -rn --include='*_test.go' 'db\.Open(' .` must return zero matches.
- Current violations: 0.

**Rule 6 [mechanical] — No `db.Store` mocks or fakes in `db/` tests.**

- Why: The whole point of running against real in-memory SQLite is to exercise the real SQL and schema. Mocks in the foundation layer would hide the failure modes this test suite exists to catch (e.g., migration regressions, constraint violations).
- Verify: `grep -rn 'type \(mock\|fake\|Mock\|Fake\)\w*' db/` must return zero matches.
- Current violations: 0.

**Rule 7 [agent] — Test functions use table-driven patterns (`tests := []struct{ name string; ... }`) when exercising multiple cases.**

- Why: Consistency of test style makes regressions easier to locate and helpers easier to share. Table-driven tests also naturally separate data from assertions.
- Verify: Agent judgment. The periodic GC action inspects `*_test.go` for repeated if-chain test bodies that should be collapsed into a table.
- Current violations: not quantified (agent-checked).

### Error handling

**Rule 8 [enforced] — Errors are wrapped with `fmt.Errorf("context: %w", err)` when propagated.**

- Why: Preserves `errors.As`/`errors.Is` at call-site boundaries without inventing custom types for every layer.
- Verify: `errorlint` is enabled in `.golangci.yml`. Run `golangci-lint run ./...`.
- Current violations: 0.

**Rule 9 [mechanical] — Custom error types (any `type *Error struct`) MUST implement `Unwrap() error`.**

- Why: Rule 8 relies on unwrapping. A custom error type without `Unwrap` silently breaks `errors.As`/`errors.Is` at the outermost frame (see `dispatch.ActionFailedError` as the canonical example at `dispatch/execute.go:48`).
- Verify: For each type matched by `grep -rn 'type \w*Error struct'`, confirm an `Unwrap() error` method exists in the same package.
- Current violations: 0 (1 type: `dispatch.ActionFailedError`, verified to implement `Unwrap`).

### Metadata access

**Rule 10 [mechanical] — Action/task metadata keys are accessed via the `MetaKey*` constants in `dispatch/execute.go`, not via string literals.**

- Why: Metadata is JSON blob storage; literal keys scattered across layers become typo vectors and make key renames painful. Constants localize the allowlist at the dispatch boundary — mirrors the article's "validate at boundaries" example.
- Verify: `grep -rn 'metadata\["' cmd/ dispatch/ tui/` must return zero matches.
- Current violations: 0.

### SQL placement

**Rule 11 [mechanical] — Raw SQL string literals live only in `db/`. No `UPDATE`, `INSERT`, `DELETE`, `SELECT`, or `CREATE TABLE` strings in `cmd/`, `dispatch/`, or `tui/` (including tests).**

- Why: If upper-layer tests bypass `db.Store` to tweak rows directly (e.g., setting `created_at` for time-travel tests), they escape the interface contract. Schema changes silently break those tests, and the interface stops being a meaningful boundary. The fix is to extend `db.Store` with the test-seam methods those tests need (e.g., `SetActionStartedAt`) and route tests through the interface.
- Verify: `grep -rn -E '"(SELECT|INSERT |UPDATE |DELETE FROM|CREATE TABLE)' cmd/ dispatch/ tui/` must return zero matches.
- Current violations: **30** across 4 files:
  - `dispatch/queue_worker_test.go`: 17 matches (all `d.Exec("UPDATE actions SET ...")` for setting `session_id`, `tmux_pane`, `started_at`)
  - `dispatch/schedule_test.go`: 7 matches (all `d.Exec("UPDATE schedules SET created_at = ...")`)
  - `tui/tasks_test.go`: 5 matches (all `d.Exec("UPDATE ... SET created_at = ...")`)
  - `cmd/reset_test.go`: 1 match (`d.Exec("UPDATE actions SET status = 'open' WHERE id = 1")`)
- Remediation sketch (to be handled by Phase 2 GC action, not this PR): expose narrow test-seam methods on `db.Store` — e.g., `db.SetActionTimestampsForTest`, `db.SetScheduleCreatedAtForTest` — guarded by a build tag or a `TestHelper` sub-interface. All 30 call sites then route through those methods.

### CLI output shape

**Rule 12 [mechanical] — `cmd/` list and show commands output JSON via the shared `WriteJSON` helper. No `fmt.Println` for structured data.**

- Why: tq's CLI contract is JSON + `--jq` for filtering. Any command that prints structured data via `fmt.Println` breaks `--jq` and forces downstream callers to parse ad-hoc text.
- Verify: `grep -rn 'fmt\.Println' cmd/` must return zero matches for structured-output cases. (Plain progress or error messages going to stderr are fine, but no cmd/ file currently uses `fmt.Println` at all, which is a cleaner state than the rule requires.)
- Current violations: 0.

---

## How to use this file

**During normal development:**

- When writing new code, skim the table of contents above (rule titles only). Rules marked `[enforced]` will block merges automatically; treat `[mechanical]` and `[agent]` rules as habits the reviewer will look for.
- When a rule gets in your way, the correct move is to **question the rule, not to work around it silently**. Open a PR that edits this file with the rationale for deleting or relaxing the rule, and let the reviewer decide.
- Do **not** add new rules based on personal taste. A rule belongs here only if you can write its `Verify` column as a runnable command or a specific agent check.

**During periodic GC (Phase 2 / 3, not yet wired):**

- A weekly GC action scans the repo against each `[mechanical]` rule, and runs agent checks for `[agent]` rules.
- Violations are appended to `docs/tech-debt-tracker.md` (also a Phase 2 deliverable) with file path, rule number, and a one-line reason.
- The GC action auto-opens targeted refactor PRs — each small enough to review in under a minute — that convert a single violation into a compliant form.

**Adding a new rule:**

1. The rule MUST have a `Verify` column with a runnable command or a precise agent check. "Looks cleaner" is not a verify clause.
2. Add the rule under the relevant heading. Preserve the numbering; never reuse a number after deletion.
3. Record `current violations: N` at the time of introduction. A non-zero N is fine — the rule still captures intent and becomes Phase 2 GC fuel.
4. No update to `CLAUDE.md` is needed — the single-line pointer in CLAUDE.md already resolves here.

---

## Per-layer quality grades

Tracking placeholder for Phase 2. The GC action will populate this matrix weekly. A cell is `OK` if the rule has zero violations in that layer, or `N` (the current violation count) otherwise. `—` means the rule does not apply to that layer.

| Rule | db | dispatch | tui | cmd |
|---|---|---|---|---|
| 1 Layer direction | OK | OK | OK | OK |
| 2 testutil isolation | — | — | — | — |
| 3 dispatch/tui cross-import | — | OK | OK | — |
| 4 `db.Store` interface | — | OK | OK | OK |
| 5 `testutil.NewTestDB` | OK | OK | OK | OK |
| 6 No db mocks | OK | — | — | — |
| 7 Table-driven tests | _agent_ | _agent_ | _agent_ | _agent_ |
| 8 Error wrapping | OK | OK | OK | OK |
| 9 Custom error Unwrap | — | OK (1) | — | — |
| 10 Metadata via constants | — | OK | OK | OK |
| 11 SQL in db/ only | — | **24** | **5** | **1** |
| 12 CLI WriteJSON | — | — | — | OK |

Totals (as of this file's introduction): **30** current violations, all against Rule 11, all in test files.
