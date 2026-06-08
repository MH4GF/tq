# tq — Task Queue CLI/TUI

Run `tq --help` for data model, commands, and usage examples.

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Single test: `go test ./db/ -run TestTaskCreate`
- Lint: `golangci-lint run` (CI also runs `./scripts/deadcode-check.sh` — Rule 13)
- Install: `go install .`

`tq` CLI subcommands (see `docs/cli-reference.md` for flags and examples):

- `tq project` / `tq task` / `tq action` — manage projects, tasks, and actions
- `tq schedule` — create and manage scheduled actions
- `tq event` — query the event log
- `tq search <keyword>` — full-text search across task titles, task metadata, task status change reasons, action titles, action results, and action metadata
- `tq config` — get/set global settings stored in the DB (e.g. `default_mode`)
- `tq ui` — launch interactive TUI with queue worker
- `tq completion` — generate shell autocompletion scripts

## Style

- Table-driven tests in `*_test.go`; use `testutil.NewTestDB()` for in-memory SQLite (or `testutil.NewFileTestDB()` when the test needs the multi-connection pool, e.g. SQLITE_BUSY contention)
- Wrap errors with `fmt.Errorf`

## Pointers

- Lint rules: `.golangci.yml`
- Architecture layers enforced by depguard: db (foundation) ← dispatch/tui (service) ← cmd (orchestrator) ← main
- Golden rules (verifiable architectural constraints): `docs/golden-rules.md`

## Release

- All `.claude-plugins/*/.claude-plugin/plugin.json` files MUST share the same `version` as `cmd/version.go`; tagpr only bumps files matching the prior version string.
- Do not set `version` in `.claude-plugin/marketplace.json` plugin entries — `plugin.json` is the single source of truth.
- Release flow is automated by tagpr; the bump target list lives in `.tagpr`.

## Quality Gates

- MUST: Run `/quality-review` before marking work complete
- Requires `flock` on `PATH` (macOS: `brew install flock`) — `.claude/scripts/record-quality-review.sh` uses it to serialize the state ledger across worktrees
- Enforced: a PreToolUse hook (`.claude/hooks/check-quality-review.sh`) blocks `gh pr create` until `/quality-review` has recorded the current HEAD SHA
- Enforced: a Stop hook (`.claude/hooks/check-session-completion.sh`) pushes the agent back if the session is about to end with uncommitted changes, commits ahead of origin/main without a PR, or an open PR (failing/pending CI, draft, conflict, or simply un-merged). Follow the slash command quoted in the push-back message, then retry.
- Auto: a PostToolUse hook (`.claude/hooks/post-edit-go-quality.sh`) runs `golangci-lint fmt` on edited `.go` files and injects package-level lint diagnostics into the agent context (non-blocking).
