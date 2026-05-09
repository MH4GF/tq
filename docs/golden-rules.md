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
- Verify: Go test harness `internal/goldenrules/` scans for `metadata["` in `cmd/`, `dispatch/`, `tui/`. Run `go test ./internal/goldenrules/`.
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

### SQL query shape

**Rule 16 [enforced] — SQL string literals MUST NOT contain leading-wildcard `LIKE '%...'` (or `LIKE '%' || ?`) patterns.**

- Why: A `LIKE` predicate whose pattern starts with `%` cannot use a B-tree index and degrades to a full table scan. On Turso this is billed in rows-read; `turso db inspect tq --queries` Top #4 (`actions WHERE metadata LIKE '%permission_mode%' OR LIKE '%worktree%'`, 2.2M reads) and Top #6 (`db.Search` UNION ALL, 235K reads) are the canonical offenders.
- Verify: Go test harness `internal/goldenrules/` scans `db/`, `cmd/`, `dispatch/`, `tui/` for the regex `(?i)\bLIKE\s+'%` (case-insensitive). Ceiling-based: violations below the ceiling pass, regressions fail. Run `go test ./internal/goldenrules/`.
- Why not `forbidigo`: `forbidigo` v2's visitor only descends into `*ast.Ident` and `*ast.SelectorExpr`, so it cannot match against `*ast.BasicLit` string literals. The Go test harness mirrors Rule 11's mechanism for SQL-shape rules.
- Detection limits (line-based scanner):
  - Splits across source lines (e.g., `"...LIKE\n'%foo%'..."`) escape detection.
  - Cross-literal concatenation (`"... LIKE " + "'%" + "..."`) escapes detection. Rewriting an existing violation in this shape to bypass the rule is a Rule 16 violation in spirit; reviewers should reject it.
- Current violations: 7 (ceiling: 7).
  - `db/db.go:296` — `migrateLegacyClaudeFlags` legacy data sweep (one-shot migration helper; full scan is acceptable here).
  - `db/search.go:98,101,104,107,110,118` — `db.Search` keyword search (UNION ALL of `tasks.title`, `tasks.metadata`, `actions.title`, `actions.result`, `actions.metadata`, `events.payload.reason`). FTS5 conversion is a separate task and out of scope for this rule.

### Query plan invariants

**Rule 17 [enforced] — Every SQL statement issued by `db/` MUST avoid full-table `SCAN` in `EXPLAIN QUERY PLAN`, except for entries explicitly listed in `.goldenrules-rule17-allowlist`.**

- Why: tq runs against Turso (libsql) in production where `rows-read` is metered. The Turso-recommended way to keep `rows-read` bounded is to verify each query's plan reports `SEARCH ... USING INDEX` rather than `SCAN tablename`. A missing index (e.g. `tasks(project_id)` historically) silently inflates reads; this rule structurally blocks that regression. Rule 16 covers the LIKE shape statically; Rule 17 covers the same concern dynamically (via the planner) and additionally catches cases unrelated to `LIKE` (missing FK indexes, full-table list operations, etc.). Background: `.claude/plans/ok-tq-create-action-refactored-galaxy.md` Action 4.
- Verify: Go test harness `internal/goldenrules/` (`rule17_explain_test.go`) parses `db/*.go` via `go/ast`, extracts SQL from `*.Query` / `*.QueryRow` / `*.Exec` (and the `*Context` variants), runs `EXPLAIN QUERY PLAN` against `testutil.NewTestDB(t)`, and matches `^SCAN\s+\w+` in the `detail` column. Ceiling-based against `.goldenrules-rule17-allowlist` (deadcode-check pattern): new findings AND stale entries both fail. Run `go test ./internal/goldenrules/`.
- Allowlist policy: `.goldenrules-rule17-allowlist` records intentional / unavoidable SCAN sites (full-table list with no WHERE, migration one-shots, FK-cascade chains that lack an index, leading-wildcard `LIKE` searches that are already tracked separately for FTS5 migration). Each entry is one line, format `<file>:<line> SCAN <table_or_alias>`. New SCANs MUST be either fixed (add an index, rewrite the query) or, if genuinely unavoidable, added to the allowlist with a category comment block above.
- MVP limits (extend in a follow-up if violated by a real refactor):
  - SQL extractor handles string literals, package-level `const` references, intra-function string variable assignments (first assignment wins), `+` concatenation, and `fmt.Sprintf` format strings (verbs replaced with `?`). It does NOT trace through helpers like `appendOrderLimit` — only the base assignment is checked.
  - Queries built via `strings.Join` for IN-clauses are checked with a single `?` placeholder substituted in. This is the worst case for the index decision.
  - Queries that fail `EXPLAIN QUERY PLAN` (typically migration queries against post-migration schema, e.g. dropped columns) are skipped with a count logged in the test output.
  - The output of `EXPLAIN QUERY PLAN` is SQLite-version dependent; the test pins itself to the in-memory SQLite shipped with `testutil.NewTestDB(t)` to keep CI deterministic.
- Current violations: 13 (see `.goldenrules-rule17-allowlist`). Burning these down (FTS5 for search, `schedules(task_id)` index, etc.) is tracked as separate follow-up actions.

---

## How to use this file

**During normal development:**

- When writing new code, skim the table of contents above (rule titles only). Rules marked `[enforced]` will block merges automatically; treat `[mechanical]` and `[agent]` rules as habits the reviewer will look for.
- When a rule gets in your way, the correct move is to **question the rule, not to work around it silently**. Open a PR that edits this file with the rationale for deleting or relaxing the rule, and let the reviewer decide.
- Do **not** add new rules based on personal taste. A rule belongs here only if you can write its `Verify` column as a runnable command or a specific agent check.

**During periodic GC (`/gc-golden-rules`, weekly via tq schedule):**

- Enforced rules (1-6, 8-14, 16-17) are checked by CI on every push/PR. The GC command covers only agent-judgment checks: Rule 7 (table-driven tests) and documentation drift.
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
| 11 SQL in db/ only | — | OK | OK | OK |
| 12 CLI WriteJSON | — | — | — | OK |
| 13 No dead code | OK | OK | OK | OK |
| 14 No `*ForTest` in prod | — | OK | OK | OK |
| 16 No leading-wildcard `LIKE` | 7 | OK | OK | OK |
| 17 No SCAN in EXPLAIN | 13 | — | — | — |

Totals: **20** current violations (Rule 16: 7 ceiling-pinned LIKE patterns; Rule 17: 13 SCANs allowlisted in `.goldenrules-rule17-allowlist`; both burn down via FTS5 conversion of `db.Search` and per-query index work tracked as separate actions).

---

## Exploratory review (rule discovery)

`/gc-exploratory` is the upstream feeder for this rule list. Where the rules above encode invariants we already know we want, the exploratory pass roams the codebase with deliberately vague intent ("find anything that concerns you") and surfaces concerns that no lint, no golden-rule, and no docs-reviewer can catch.

It runs as Phase 3 of `/gc-golden-rules` and turns each verified concern into a child tq action on the same task.

**When a class of concern shows up repeatedly, that is the signal to lift it into a verifiable rule here.** This is the feedback loop that grows the rule list over time.
