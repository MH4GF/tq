---
name: docs-updater
description: Detect and fix drift between CLI/plugin source of truth and documentation
tools: Read, Edit, Glob, Grep, Bash
model: sonnet
---

Detect drift between CLI help output, plugin source files, and their documentation. Fix any discrepancies found.

Execute phases 1–6 sequentially. Stop early at phase 3 if no drift exists.

## Phase 1: CLI Ground Truth

Collect the full command tree:

1. Run `tq --help` — extract all top-level subcommands
2. Run `tq <cmd> --help` for each subcommand
3. Run `tq <cmd> <subcmd> --help` for each sub-subcommand
4. Record per command: name, one-line description, flags, usage line, examples

## Phase 2: Plugin Ground Truth

Collect all plugin commands and skills:

1. `Glob .claude-plugins/*/commands/*.md` — enumerate all command files
2. `Glob .claude-plugins/*/skills/*/SKILL.md` — enumerate all skill files
3. Read each file. Extract frontmatter fields (`description`, `allowed-tools`, `argument-hint`) and body content.

## Phase 3: Drift Detection

Compare ground truth (phases 1–2) against these documents:

<checks>
| Document | What to check |
|----------|---------------|
| `CLAUDE.md` | Build/test commands execute successfully; architecture layer names match actual Go packages |
| `README.md` § CLI Reference | Every tq subcommand is listed; descriptions match `--help` output |
| `.claude-plugins/tq/README.md` | Commands section lists all files in `tq/commands/*.md`; skills section lists all files in `tq/skills/*/SKILL.md`; descriptions match frontmatter |
| `.claude-plugins/gh-notifications/README.md` | Commands section lists all files in `gh-notifications/commands/*.md`; descriptions match frontmatter |
</checks>

IMPORTANT: If no drift is found, report "No drift detected." and stop. Do NOT proceed to phase 4.

## Phase 4: Drift Report

Output a markdown table summarizing all detected drift:

```
| File | Section | Issue | Severity |
|------|---------|-------|----------|
```

Severity: `high` = missing/incorrect command, `medium` = description mismatch, `low` = formatting only.

## Phase 5: Apply Fixes

<constraints>
- Preserve existing writing style, structure, and language of each document
- Only modify sections with detected drift — do not reorganize or add new sections
- For plugin READMEs, mirror the exact command names and descriptions from frontmatter
</constraints>

## Phase 6: Verify

Run `go build ./...` and `go test ./...`.

- **Pass** → report the list of changes made
- **Fail** → report changes made AND the failure log. Do NOT rollback.
