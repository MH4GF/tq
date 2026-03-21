# tq — Task Queue CLI/TUI

Run `tq --help` for data model, commands, and usage examples.

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Single test: `go test ./db/ -run TestTaskCreate`
- E2E test: `go test -tags e2e -count=1 ./e2e/`
- Lint: `golangci-lint run`
- Install: `go install .`

## Style

- Table-driven tests in `*_test.go`; use `testutil.NewTestDB()` for in-memory SQLite
- Wrap errors with `fmt.Errorf`

## Pointers

- Lint rules: `.golangci.yml`
