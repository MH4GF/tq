# tq — Task Queue CLI/TUI

Run `tq --help` for data model, commands, and usage examples.

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Single test: `go test ./db/ -run TestTaskCreate`
- Lint: `golangci-lint run`
- Install: `go install .`

## Style

- Table-driven tests in `*_test.go`; use `testutil.NewTestDB()` for in-memory SQLite
- Wrap errors with `fmt.Errorf`

## Pointers

- Lint rules: `.golangci.yml`
- Architecture layers enforced by depguard: db (foundation) → dispatch/tui (service) → cmd (orchestrator) → main
- Golden rules (verifiable architectural constraints): `docs/golden-rules.md`

## Quality Gates

- MUST: Run `/quality-review` before marking work complete
- Enforced: a PreToolUse hook (`.claude/hooks/check-quality-review.sh`) blocks `gh pr create` until `/quality-review` has recorded the current HEAD SHA
