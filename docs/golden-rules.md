# Golden Rules

Verifiable architectural constraints for the tq codebase.

This file is the single source of truth for agent-driven "tech-debt garbage collection" — the practice of encoding architectural invariants as mechanical rules, then running background agents to detect and auto-fix deviations. See the pattern's origin: <https://alexlavaee.me/blog/openai-agent-first-codebase-learnings/> (pattern #5).

**Rules must be verifiable constraints, not taste preferences.** Every rule below has a `Verify` column with a concrete command, lint name, or detection approach. Rules that cannot be verified do not belong here.

Classification:

- **[enforced]** — enforced mechanically by lint (depguard, forbidigo, errorlint) or Go test harness (`internal/goldenrules/`). Deviations fail CI.
- **[mechanical]** — detectable by grep / script but not yet wired into CI.
- **[agent]** — requires LLM judgment. Checked by the periodic GC action (`/gc-golden-rules`).

Current status totals are captured after each rule as `current violations: N`. A non-zero count is **not** a bug in the rule — it is tech debt to be burned down by GC actions (`/gc-golden-rules`).

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

**Rule 4 [enforced] — Upper layers depend on the `db.Store` interface, not the concrete `*db.DB` type.**

- Why: The `Store` interface (composed of `CommandWriter` + `QueryReader`, defined in `db/interfaces.go`) is the contract upper layers rely on. Depending on `*db.DB` directly leaks implementation details and blocks substitution in tests.
- Verify: Go test harness `internal/goldenrules/` scans for `*db.DB` in `cmd/`, `dispatch/`, `tui/`. Run `go test ./internal/goldenrules/`.
- Current violations: 0.

### Test strategy

**Rule 5 [enforced] — Tests use `testutil.NewTestDB(t)` (in-memory SQLite) rather than calling `db.Open` directly.**

- Why: `testutil.NewTestDB` guarantees migration is applied and cleanup is registered with `t.Cleanup`. Hand-rolled `db.Open(":memory:")` calls diverge from this and silently skip migrations.
- Verify: `forbidigo` in `.golangci.yml` blocks `db.Open` calls in `_test.go` files. Run `golangci-lint run ./...`.
- Current violations: 0.

**Rule 6 [enforced] — No `db.Store` mocks or fakes in `db/` tests.**

- Why: The whole point of running against real in-memory SQLite is to exercise the real SQL and schema. Mocks in the foundation layer would hide the failure modes this test suite exists to catch (e.g., migration regressions, constraint violations).
- Verify: Go test harness `internal/goldenrules/` scans for `type Mock*/Fake*` in `db/`. Run `go test ./internal/goldenrules/`.
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

**Rule 9 [enforced] — Custom error types (any `type *Error struct`) MUST implement `Unwrap() error`.**

- Why: Rule 8 relies on unwrapping. A custom error type without `Unwrap` silently breaks `errors.As`/`errors.Is` at the outermost frame (see `dispatch.ActionFailedError` in `dispatch/execute.go` as the canonical example).
- Verify: Go test harness `internal/goldenrules/` finds all `type *Error struct` and checks for `Unwrap() error` in the same package. Run `go test ./internal/goldenrules/`.
- Current violations: 0 (1 type: `dispatch.ActionFailedError`, verified to implement `Unwrap`).

### Metadata access

**Rule 10 [enforced] — Action/task metadata keys are accessed via the `MetaKey*` constants in `dispatch/execute.go`, not via string literals.**

- Why: Metadata is JSON blob storage; literal keys scattered across layers become typo vectors and make key renames painful. Constants localize the allowlist at the dispatch boundary — mirrors the article's "validate at boundaries" example.
- Verify: Go test harness `internal/goldenrules/` covers both reads and writes in `cmd/`, `dispatch/`, `tui/`. (1) Reader shape: line-based scan for `metadata["` catches direct `metadata["key"]` indexing. (2) Writer shape: type-driven AST scan in `rule10_metadata_test.go` flags string-literal keys in `map[string]any` composite literals passed as the 2nd arg of `db.Store.MergeActionMetadata` or as the `Updates` field of `db.ActionMetadataMerge` (the per-entry map fed to `BulkMergeActionMetadata`). The writer scan was added after `executeRemote` slipped past the reader-only regex with a literal `"remote_session"` key (the writer wasn't using `metadata[…]` syntax, so the regex saw nothing). Run `go test ./internal/goldenrules/`.
- Detection limits (writer scan): only inline composite literals are inspected — a map built into a variable then passed in (`m := map[string]any{"foo": 1}; store.MergeActionMetadata(id, m)`) is not traced through the assignment. That shape does not appear in the codebase today; if it ever does, extend the rule rather than working around it.
- Current violations: 0.

### SQL placement

**Rule 11 [enforced] — Raw SQL string literals live only in `db/`. No `UPDATE`, `INSERT`, `DELETE`, `SELECT`, or `CREATE TABLE` strings in `cmd/`, `dispatch/`, or `tui/` (including tests).**

- Why: If upper-layer tests bypass `db.Store` to tweak rows directly (e.g., setting `created_at` for time-travel tests), they escape the interface contract. Schema changes silently break those tests, and the interface stops being a meaningful boundary. The fix is to extend `db.Store` with the test-seam methods those tests need (e.g., `SetActionStartedAt`) and route tests through the interface.
- Verify: Go test harness `internal/goldenrules/` scans for SQL keywords in `cmd/`, `dispatch/`, `tui/`. Ceiling-based: violations below the ceiling pass, regressions fail. Run `go test ./internal/goldenrules/`.
- Current violations: 0 (resolved by adding `TestHelper` sub-interface with test-seam methods: `SetActionSessionInfoForTest`, `SetScheduleTimestampsForTest`, `SetActionTimestampsForTest`, `SetTaskTimestampsForTest`, `SetActionStatusForTest`).

### CLI output shape

**Rule 12 [enforced] — `cmd/` list and show commands output JSON via the shared `WriteJSON` helper. No `fmt.Println` for structured data.**

- Why: tq's CLI contract is JSON + `--jq` for filtering. Any command that prints structured data via `fmt.Println` breaks `--jq` and forces downstream callers to parse ad-hoc text.
- Verify: `forbidigo` in `.golangci.yml` blocks `fmt.Println` calls in `cmd/`. Run `golangci-lint run ./...`.
- Current violations: 0.

### Dead code (unreachable functions)

**Rule 13 [enforced] — Functions and methods MUST be reachable from `main` or test binaries (analyzed by `golang.org/x/tools/cmd/deadcode -test`).**

- Why: `staticcheck`'s `unused` only sees within a single package. Cross-package dead code (exported functions no caller imports, methods of types that are never instantiated) slips through. tq is a closed single-binary CLI, so reachability from main + tests equals liveness.
- Verify: `scripts/deadcode-check.sh` runs `deadcode -test` and diffs the result against `.deadcode-allowlist`. Fails on **new findings** (must be fixed or allowlisted) AND on **stale allowlist entries** (must be removed). The CI `lint` job runs the script. Run `./scripts/deadcode-check.sh`.
- Allowlist policy: only intentional retentions belong in `.deadcode-allowlist` — interface satisfactions called via reflection, planned test seams not yet wired up, etc. Genuine dead code MUST be deleted, not allowlisted.
- Current violations: 0.

### Test seam isolation

**Rule 14 [enforced] — Test seam methods (`*ForTest`) MUST NOT be called from production code.**

- Why: `db.Store` embeds `TestHelper` (`db/interfaces.go:84-101`) so that upper-layer tests can mutate timestamps/status without writing raw SQL outside `db/` (Rule 11). The trade-off is that the test seam appears on the production `Store` API. A static lint guards the production-side boundary so the embedding does not silently expand into a production capability.
- Verify: `forbidigo` in `.golangci.yml` blocks `\.Set[A-Z][A-Za-z]*ForTest\b` outside `_test.go` files. Run `golangci-lint run ./...`.
- Current violations: 0.

### N+1 prevention

**Rule 15 [enforced] — `db.Store` methods MUST NOT be invoked inside a `for ... range` loop in `cmd/`, `dispatch/`, or `tui/`.**

- Why: A per-iteration `Store` call against a finite collection is the classic N+1 pattern. On Turso (network-backed) it directly inflates `rows-read` and round-trip latency in ways local SQLite hides. The original incident (`tui/tasks.go:144` `for _, p := range projects { m.database.ListTasksByProject(p.ID) }`) projected to 3.9B reads/month — 7.8x the free tier — before being caught. The fix is always the same: add a bulk method on `db.Store` (e.g. `GetTasksByIDs`, `BulkInsertScheduledActions`, `BulkMarkFailed`) and call it once outside the loop.
- Verify: Go test harness `internal/goldenrules/rule15_test.go` loads `cmd/`, `dispatch/`, `tui/` with `golang.org/x/tools/go/packages` (full type info), walks every `*ast.RangeStmt`, and reports any `*ast.CallExpr` whose `types.Selection` resolves to a method on the `db.Store` interface or any type implementing it. Run `go test ./internal/goldenrules/`.
- Scope of detection: `RangeStmt` only (collection iteration). Bare `for {}` and `for cond {}` polling loops (e.g. `RunWorker`'s heartbeat tick) are out of scope — each iteration is independent and bulk-batching is meaningless. The original tool considered for this rule was `masibw/goone`; it was rejected after Phase 4 of the introducing action because the upstream is unmaintained (last commit 2022-07) and breaks against modern Go's type internals (`internal error: package "context" without types ...`).
- Current violations: 0.

### SQL query shape

**Rule 16 [enforced] — SQL string literals MUST NOT contain leading-wildcard `LIKE '%...'` (or `LIKE '%' || ?`) patterns.**

- Why: A `LIKE` predicate whose pattern starts with `%` cannot use a B-tree index and degrades to a full table scan. On Turso this is billed in rows-read; `turso db inspect tq --queries` Top #4 (`actions WHERE metadata LIKE '%permission_mode%' OR LIKE '%worktree%'`, 2.2M reads) and Top #6 (`db.Search` UNION ALL, 235K reads) are the canonical offenders.
- Verify: Go test harness `internal/goldenrules/` scans `db/`, `cmd/`, `dispatch/`, `tui/` for the regex `(?i)\bLIKE\s+'%` (case-insensitive). Ceiling-based: violations below the ceiling pass, regressions fail. Run `go test ./internal/goldenrules/`.
- Why not `forbidigo`: `forbidigo` v2's visitor only descends into `*ast.Ident` and `*ast.SelectorExpr`, so it cannot match against `*ast.BasicLit` string literals. The Go test harness mirrors Rule 11's mechanism for SQL-shape rules.
- Detection limits (line-based scanner):
  - Splits across source lines (e.g., `"...LIKE\n'%foo%'..."`) escape detection.
  - Cross-literal concatenation (`"... LIKE " + "'%" + "..."`) escapes detection. Rewriting an existing violation in this shape to bypass the rule is a Rule 16 violation in spirit; reviewers should reject it.
- Current violations: 0 (ceiling: 0).
  - `db.Search` (all 6 branches) was converted to SQLite FTS5 (`trigram` tokenizer) virtual tables (`tasks_fts`/`actions_fts`/`events_fts`), kept in sync by `AFTER INSERT/UPDATE/DELETE` triggers and an idempotent backfill in `db.go` `Migrate()`. The leading-wildcard `LIKE` is gone; the query is now index-backed (`SCAN <fts> VIRTUAL TABLE INDEX`). trigram preserves substring search for ≥3-rune keywords incl. Japanese; 1–2-rune queries no longer match (proper CJK morphological tokenization is separate follow-up work). The last remaining offender (`migrateLegacyClaudeFlags` one-shot sweep) was already removed in a prior burndown, so Rule 16 is now fully clean.

### Query plan invariants

**Rule 17 [enforced] — Every SQL statement issued by `db/` MUST avoid full-table `SCAN` in `EXPLAIN QUERY PLAN`, except for entries explicitly listed in `.goldenrules-rule17-allowlist`.**

- Why: tq runs against Turso (libsql) in production where `rows-read` is metered. The Turso-recommended way to keep `rows-read` bounded is to verify each query's plan reports `SEARCH ... USING INDEX` rather than `SCAN tablename`. A missing index (e.g. `tasks(project_id)` historically) silently inflates reads; this rule structurally blocks that regression. Rule 16 covers the LIKE shape statically; Rule 17 covers the same concern dynamically (via the planner) and additionally catches cases unrelated to `LIKE` (missing FK indexes, full-table list operations, etc.). Background: `.claude/plans/ok-tq-create-action-refactored-galaxy.md` Action 4.
- Verify: Go test harness `internal/goldenrules/` (`rule17_explain_test.go`) parses `db/*.go` via `go/ast`, extracts SQL from `*.Query` / `*.QueryRow` / `*.Exec` (and the `*Context` variants), runs `EXPLAIN QUERY PLAN` against `testutil.NewTestDB(t)`, and matches `^SCAN\s+\w+` in the `detail` column. Ceiling-based against `.goldenrules-rule17-allowlist` (deadcode-check pattern): new findings AND stale entries both fail. Run `go test ./internal/goldenrules/`.
- Allowlist policy: `.goldenrules-rule17-allowlist` records intentional / unavoidable SCAN sites (full-table list with no WHERE, migration one-shots, FK-cascade chains that lack an index, FTS5 `MATCH` virtual-table index scans, LIMIT-bounded primary-key scans where `ORDER BY id DESC LIMIT N` is actually O(N) and no index can improve it). Each entry is one line, format `<file>:<line> SCAN <table_or_alias>`. New SCANs MUST be either fixed (add an index, rewrite the query) or, if genuinely unavoidable, added to the allowlist with a category comment block above.
- MVP limits (extend in a follow-up if violated by a real refactor):
  - SQL extractor handles string literals, package-level `const` references, intra-function string variable assignments (first assignment wins), `+` concatenation, and `fmt.Sprintf` format strings (verbs replaced with `?`). It does NOT trace through helpers like `appendOrderLimit` — only the base assignment is checked.
  - Queries built via `strings.Join` for IN-clauses are checked with a single `?` placeholder substituted in. This is the worst case for the index decision.
  - Queries that fail `EXPLAIN QUERY PLAN` (typically migration queries against post-migration schema, e.g. dropped columns) are skipped with a count logged in the test output.
  - The output of `EXPLAIN QUERY PLAN` is SQLite-version dependent; the test pins itself to the in-memory SQLite shipped with `testutil.NewTestDB(t)` to keep CI deterministic.
- Current violations: 16 allowlist entries across 8 sites (see `.goldenrules-rule17-allowlist`). `db.Search` is now FTS5 (its scans are the index-backed `SCAN <fts> VIRTUAL TABLE INDEX` path, not full scans). Remaining burndown is tracked as separate follow-up actions.

### Aggregate queries on hot paths

**Rule 18 [enforced] — `tui/` and `dispatch/` MUST NOT contain `SELECT COUNT/SUM/AVG` string literals. Aggregate-driven counts on `actions` go through `db.Store.GetTaskActionCount`, which reads from the trigger-maintained `task_action_counts` table.**

- Why: `turso db inspect tq --queries` repeatedly surfaces `SELECT COUNT(*) FROM actions WHERE task_id = ? AND status IN (...)` in the top-N rows-read consumers. Turso bills per row read, so per-tick aggregate scans dominate quota. The `task_action_counts(task_id, status, count)` table is maintained by `AFTER INSERT / AFTER UPDATE OF status / AFTER DELETE` triggers on `actions`, so any task's status counts are a 1-row index lookup. This rule structurally bans hot-path code from re-introducing aggregate scans — it is a specialization of Rule 11 narrowed to the most expensive aggregates.
- Assumption: `actions.task_id` is immutable. Triggers do not handle `task_id` changes; no production path issues `UPDATE actions SET task_id = ?`. If that changes, add a trigger or migrate the data manually.
- Backfill: `db.Migrate()` runs an idempotent `INSERT INTO task_action_counts SELECT task_id, status, COUNT(*) FROM actions GROUP BY task_id, status` only when `task_action_counts` is empty (`backfillTaskActionCounts` in `db/db.go`). Subsequent runs see existing rows kept in sync by the triggers and skip the backfill.
- Verify: Go test harness `internal/goldenrules/` scans `tui/`, `dispatch/` for `"...SELECT COUNT|SUM|AVG..."` string literals. Ceiling-based: violations below the ceiling pass, regressions fail. Run `go test ./internal/goldenrules/`.
- Detection limits (same as Rule 11/16): line-split (`"SELECT " + "COUNT(*)"`) and cross-literal concatenation (`prefix + " COUNT(*)"`) bypass detection. Reviewers MUST reject rewrites that exploit these to hide hot-path aggregates.
- Current violations: 0.
- Runtime counterpart: Rules 11/16/18 are PR-time and method-name dependent; the weekly `/turso-query-watch` schedule observes the real metered `rows-read` from `turso db inspect tq --queries` and files an action on regression, catching what slips past these static guards. See `docs/turso-query-watch.md`.

### Test-only-reachable interface methods

**Rule 19 [enforced] — Every `db.Store` interface method (except the intentionally test-only `TestHelper` sub-interface) MUST have at least one non-test caller. A method whose only caller is its own self-test is dead from production.**

- Why: Rule 13 (`deadcode -test`) cannot see this class of dead code. `deadcode` builds an RTA call graph; once `*db.DB` is instantiated and used as `db.Store` in production, RTA conservatively marks every interface-dispatched method reachable, and the `-test` flag additionally keeps any method its self-test exercises "live". So a `db.Store` method that lost its last production caller (incident: `db.Store.ListTasksByProject` after PR #261, whose sole remaining caller was `TestListTasksByProject`; removed by PR #267) stays green under Rule 13. Rule 19 closes that specific blind spot for the `db.Store` contract.
- Scope: `db.Store` only (the foundational contract every upper layer depends on — Rule 4). Other interfaces are deliberately out of scope to keep the check tight and false-positive-free.
- Why this mechanism (Go test harness, not a second `deadcode` pass): a `deadcode` run *without* `-test` inherits the exact same RTA interface-dispatch over-approximation as Rule 13 and would NOT report these methods either (verified empirically: `deadcode ./...` flags none of them). A second-pass/diff approach is therefore both noisier and unsound for this question. The type-driven AST scan mirrors Rule 15's proven `go/types` + `golang.org/x/tools/go/packages` pattern (no new dependency). `TestHelper` is excluded structurally: the check resolves the `db.TestHelper` interface via `go/types` and subtracts its method-name set, so test seams never need allowlisting.
- "Non-test caller" spans the whole module, not just `cmd/`/`dispatch/`/`tui/`/`main.go`: a `db.Store` method invoked internally by another `db/` method (e.g. `GetTaskActionCount` called from `db/task.go`) is exercised in production transitively, so restricting the scan to upper layers would produce false positives. The rule fires only when there is zero non-`_test.go` caller anywhere.
- Verify: Go test harness `internal/goldenrules/rule19_test.go`. Ceiling-based against `.goldenrules-rule19-allowlist` (deadcode-check discipline): new dead methods AND stale allowlist entries (method regained a non-test caller or was removed from `db.Store`) both fail. Run `go test ./internal/goldenrules/ -run TestRule19_NoTestOnlyStoreMethods`.
- Allowlist policy: only deliberate seams belong in `.goldenrules-rule19-allowlist` long-term. The current entries are a pre-existing blind-spot backlog surfaced when the rule was introduced; each is tracked by a dedicated burn-down action on task #660 (resolve = delete the dead method or wire a real production caller, then remove the line).
- Current violations: 0. The pre-existing blind-spot backlog surfaced when the rule was introduced has been fully burned down (each method either deleted as dead or wired to a real production caller via task #660); `.goldenrules-rule19-allowlist` is now empty.

---

## How to use this file

**During normal development:**

- When writing new code, skim the table of contents above (rule titles only). Rules marked `[enforced]` will block merges automatically; treat `[mechanical]` and `[agent]` rules as habits the reviewer will look for.
- When a rule gets in your way, the correct move is to **question the rule, not to work around it silently**. Open a PR that edits this file with the rationale for deleting or relaxing the rule, and let the reviewer decide.
- Do **not** add new rules based on personal taste. A rule belongs here only if you can write its `Verify` column as a runnable command or a specific agent check.

**During periodic GC (`/gc-golden-rules`, weekly via tq schedule):**

- Enforced rules (1-6, 8-19) are checked by CI on every push/PR. The GC command covers only agent-judgment checks: Rule 7 (table-driven tests) and documentation drift.
- For each violation found, the GC command creates a tq action via `/tq:create-action` with `claude_args: ["--worktree"]` for isolated execution.
- The created actions handle the actual fixes — each targeted to a single violation.

**Adding a new rule:**

1. The rule MUST have a `Verify` column with a runnable command or a precise agent check. "Looks cleaner" is not a verify clause.
2. Add the rule under the relevant heading. Preserve the numbering; never reuse a number after deletion.
3. Record `current violations: N` at the time of introduction. A non-zero N is fine — the rule still captures intent and the GC action will create tq actions to burn it down.
4. No update to `CLAUDE.md` is needed — the single-line pointer in CLAUDE.md already resolves here.
5. Choose the enforcement method by priority:
   1. **Existing linter** (depguard, forbidigo, errorlint, etc.) — widely used, stable, and already integrated into CI. Prefer this when a linter can express the rule.
   2. **Go test harness** (`internal/goldenrules/goldenrules_test.go`) — runs via `go test ./...` in CI. Use for rules that need file scanning or cross-file correlation beyond what linters support.
   3. **Custom `go/analysis` analyzer** — powerful but heavy to implement. Use only as a last resort.

---

## Per-layer quality grades

A cell is `OK` if the rule has zero violations in that layer, or `N` (the current violation count) otherwise. `—` means the rule does not apply to that layer.

Rule 2's scope (`testutil/` MUST NOT import upward) sits outside this per-layer view — the importer is `testutil/`, not any of the columns — so its row is all `—`. Status is tracked in the rule body above.

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
| 9 Custom error Unwrap | — | OK | — | — |
| 10 Metadata via constants | — | OK | OK | OK |
| 11 SQL in db/ only | — | OK | OK | OK |
| 12 CLI WriteJSON | — | — | — | OK |
| 13 No dead code | OK | OK | OK | OK |
| 14 No `*ForTest` in prod | — | OK | OK | OK |
| 15 No N+1 in for-range | — | OK | OK | OK |
| 16 No leading-wildcard `LIKE` | OK | OK | OK | OK |
| 17 No SCAN in EXPLAIN | 16 | — | — | — |
| 18 No aggregate hot paths | — | OK | OK | — |
| 19 No test-only `db.Store` methods | 0 | — | — | — |

Totals: **16** current violations (Rule 16: 0 — `db.Search` burned down via FTS5 trigram conversion and the `migrateLegacyClaudeFlags` one-shot already removed, so Rule 16 is fully clean; Rule 17: 16 SCANs allowlisted in `.goldenrules-rule17-allowlist`, of which the `db.Search` entries are the index-backed FTS5 virtual-table path; Rule 19: 0 — fully burned down, `.goldenrules-rule19-allowlist` is empty; Rule 17 remaining burns down via per-query index work tracked as separate actions).

---

## Exploratory review (rule discovery)

`/gc-exploratory` is the upstream feeder for this rule list. Where the rules above encode invariants we already know we want, the exploratory pass roams the codebase with deliberately vague intent ("find anything that concerns you") and surfaces concerns that no lint, no golden-rule, and no docs-reviewer can catch.

It runs as Phase 3 of `/gc-golden-rules` and turns each verified concern into a child tq action on the same task.

**When a class of concern shows up repeatedly, that is the signal to lift it into a verifiable rule here.** This is the feedback loop that grows the rule list over time.
