---
name: e2e-execute
description: Execute diff-scoped E2E tests against the built tq binary; report PASS/FAIL/SKIPPED with raw output as evidence
tools: Read, Glob, Grep, Bash
model: sonnet
---

Run diff-scoped E2E against the freshly built `tq` binary and report PASS / FAIL / SKIPPED with raw command output as evidence.

This agent is part of `/quality-review`. It is the only check that exercises the real binary, so its output **must contain executable evidence** (commands + their stdout/stderr), not summaries.

Execute phases 1–4 sequentially. Stop early at phase 2 if scope is SKIPPED.

## Phase 1: Diff capture

1. Run `git diff --name-only origin/main...HEAD` — full list of changed files
2. If any `cmd/*.go` (excluding `*_test.go`) is changed, also run `git diff origin/main...HEAD -- cmd/` to read what actually changed (used to design fallback sequences)

## Phase 2: Scope classification

Walk these checks in order. The first one that matches decides the action.

1. **EXECUTE** — any `cmd/*.go` (excluding `*_test.go`) is in the diff. Proceed to Phase 3.
2. **SKIPPED** — the diff consists *only* of paths matching one of the categories below. Emit the listed reason.
   - `docs/`, `*.md`, `.golangci.yml`, `.github/`, `.claude/`, `.claude-plugins/` → `docs/config only`
   - `*_test.go` → `tests only`
   - `db/*.go` (siblings: `*_test.go`, `docs/`, `*.md` ok) → `db/ covered by go test ./db/ with in-memory SQLite`
   - `dispatch/*.go` (same siblings) → `dispatch/ covered by go test ./dispatch/, loop integration not feasible in this agent`
   - `internal/goldenrules/*.go` (same siblings) → `go test ./internal/goldenrules/ is the e2e`
3. **SKIPPED** — fallback for any diff that does not match rules 1 or 2. Emit reason `no binary-affecting cmd/ change`.

**TUI side-channel (independent of the rules above):** if any `tui/*.go` is in the diff, append a manual checklist to the report (e.g., "launch `tq ui`, verify list renders / arrow keys / status filter"). Do this whether the decision was EXECUTE or SKIPPED.

If the decision is SKIPPED, jump to Phase 4 and emit the reason.

## Phase 3: Execute (cmd/ changes only)

If `cmd/ui.go` is among the changed files, treat it like `tui/*.go`: route to the manual checklist (the TUI side-channel) and skip the binary sequences below — automated coverage is not feasible.

### Setup + sequences (run as a SINGLE bash invocation)

Run setup and every standard sequence inside one bash invocation so shell variables (`DB`, `BIN`) survive between commands. Do NOT split the setup into multiple Bash tool calls — `$$` resolves to a different PID each time and the variables vanish.

```bash
set -e
rm -f /tmp/tq-qr.db
go build -o /tmp/tq-qr . || { echo "FAIL: go build failed"; exit 1; }
DB=/tmp/tq-qr.db
BIN="/tmp/tq-qr --db $DB"
$BIN project create qr-test /tmp
# … then append the matching standard sequence(s) below in the same bash block
```

The `--db` flag isolates the user's real DB; `/tmp/tq-qr` keeps the binary out of `$GOPATH/bin`.

### Standard sequences

For each changed `cmd/*.go` file (non-test), append the matching sequence below to the bash block. Multiple changes → append all matching sequences (sharing the same DB is fine; each sequence creates the rows it needs).

| Changed cmd | Standard sequence |
|---|---|
| `cmd/task.go` | `task create … --project 1` → `task list` → `task update <id> …` → `task list` |
| `cmd/action.go`, `cmd/add.go` | `task create` → `action create` → `action list` |
| `cmd/done.go` | `action create` → `action done <id>` → `action list --status done` |
| `cmd/cancel.go` | `action create` → `action cancel <id>` → `action list --status cancelled` |
| `cmd/fail.go` | `action create` → `action fail <id>` → `action list --status failed` |
| `cmd/dispatch.go` | SKIP this sequence (`dispatch loop requires tmux/worker env`) — note in output |
| `cmd/list.go`, `cmd/search.go` | seed data (task + action) → run the list/search command → verify with `jq` (count or fields) |
| `cmd/schedule.go` | `schedule create` → `schedule list` → `schedule delete` |
| `cmd/project.go` | `project create` → `project list` |
| Anything else (e.g. `attach.go`, `event.go`, `jq.go`, `reset.go`, `validate.go`, `version.go`, `helputil.go`, `root.go`) | Read the diff and `$BIN <subcommand> --help`, then design a minimal 1–3 command sequence that exercises the changed surface |

### Execution rules

- Run each command in the foreground; capture stdout and stderr.
- Check exit code after every command. **Any non-zero exit → FAIL** (skip remaining commands, jump to Phase 4).
- Confirm `--help` for the changed subcommand actually lists the changed flag/option, when applicable.
- For list/search verification, use `jq` to assert structure (e.g., `jq '. | length'`, `jq '.[0].status'`). Reject silent shape regressions.

## Phase 4: Output

Start the report with one of the conclusion lines:

- `**結論: PASS**`
- `**結論: FAIL**`
- `**結論: SKIPPED**`

Then:

**PASS** — list every command run and its raw stdout/stderr. This is the evidence the parent agent uses to record the quality-review SHA. No summarizing.

**FAIL** — show the failing command, its full stderr/stdout, and the exit code. Stop on the first failure; do not continue running sequences.

**SKIPPED** — print the reason from Phase 2 (1–2 lines is enough; line-level analysis is not required). For `tui/` changes, append a short manual checklist for the user.

IMPORTANT: Do NOT apply fixes. Do NOT edit files. Report only — the parent `/quality-review` decides next steps.
