# Changelog

## [v0.21.13](https://github.com/MH4GF/tq/compare/v0.21.12...v0.21.13) - 2026-04-17
- Add gc-golden-rules periodic GC command (Phase 3) by @MH4GF in https://github.com/MH4GF/tq/pull/109
- quality-review: fix code-simplifier agent type not found error by @MH4GF in https://github.com/MH4GF/tq/pull/111
- Move dispatch context from preamble to postamble by @MH4GF in https://github.com/MH4GF/tq/pull/104
- Replace text help instruction with auto-exec in manager SKILL.md by @MH4GF in https://github.com/MH4GF/tq/pull/112
- gh-ops:watch の gh api /repos 呼び出しを専用サブコマンドに置換 by @MH4GF in https://github.com/MH4GF/tq/pull/113
- Include CLI output in noninteractive worker error messages by @MH4GF in https://github.com/MH4GF/tq/pull/117
- Fix noninteractive actions not saving claude_session_id by @MH4GF in https://github.com/MH4GF/tq/pull/102
- Add work_dir auto-recovery for stale worktree paths by @MH4GF in https://github.com/MH4GF/tq/pull/118

## [v0.21.12](https://github.com/MH4GF/tq/compare/v0.21.11...v0.21.12) - 2026-04-13
- Add distinct gray styling for archived/cancelled statuses in TUI by @MH4GF in https://github.com/MH4GF/tq/pull/87
- Revise README and add CLI reference docs by @MH4GF in https://github.com/MH4GF/tq/pull/79
- Skip investigate-failure for all timeout failures by @MH4GF in https://github.com/MH4GF/tq/pull/90
- Add skip condition for team review requests in watch by @MH4GF in https://github.com/MH4GF/tq/pull/91
- Add --db flag and TQ_DB_PATH env var for DB path override by @MH4GF in https://github.com/MH4GF/tq/pull/96
- Fix tagpr versionFile path after gh-ops rename by @MH4GF in https://github.com/MH4GF/tq/pull/95
- Add tmp file draft workflow to respond-review by @MH4GF in https://github.com/MH4GF/tq/pull/93
- Detect permission_denials in noninteractive worker by @MH4GF in https://github.com/MH4GF/tq/pull/94
- Add tq action fail CLI and /tq:failed slash command by @MH4GF in https://github.com/MH4GF/tq/pull/92
- Add dispatch_after for scheduled action dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/89
- Add missing dispatchAfter arg to InsertAction calls by @MH4GF in https://github.com/MH4GF/tq/pull/100
- Add demo GIF to README by @MH4GF in https://github.com/MH4GF/tq/pull/99
- Add automated golden rule violation detection (Phase 2) by @MH4GF in https://github.com/MH4GF/tq/pull/101
- Resolve all 30 Rule 11 violations via db.Store test-seam methods by @MH4GF in https://github.com/MH4GF/tq/pull/106
- Add claude_args metadata key for dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/105
- Show available projects in task create help by @MH4GF in https://github.com/MH4GF/tq/pull/103
- Add Homebrew cask distribution via GoReleaser by @MH4GF in https://github.com/MH4GF/tq/pull/107
- Release for v0.21.12 by @MH4GF in https://github.com/MH4GF/tq/pull/98

## [v0.21.12](https://github.com/MH4GF/tq/compare/v0.21.11...v0.21.12) - 2026-04-13
- Add distinct gray styling for archived/cancelled statuses in TUI by @MH4GF in https://github.com/MH4GF/tq/pull/87
- Revise README and add CLI reference docs by @MH4GF in https://github.com/MH4GF/tq/pull/79
- Skip investigate-failure for all timeout failures by @MH4GF in https://github.com/MH4GF/tq/pull/90
- Add skip condition for team review requests in watch by @MH4GF in https://github.com/MH4GF/tq/pull/91
- Add --db flag and TQ_DB_PATH env var for DB path override by @MH4GF in https://github.com/MH4GF/tq/pull/96
- Fix tagpr versionFile path after gh-ops rename by @MH4GF in https://github.com/MH4GF/tq/pull/95
- Add tmp file draft workflow to respond-review by @MH4GF in https://github.com/MH4GF/tq/pull/93
- Detect permission_denials in noninteractive worker by @MH4GF in https://github.com/MH4GF/tq/pull/94
- Add tq action fail CLI and /tq:failed slash command by @MH4GF in https://github.com/MH4GF/tq/pull/92
- Add dispatch_after for scheduled action dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/89
- Add missing dispatchAfter arg to InsertAction calls by @MH4GF in https://github.com/MH4GF/tq/pull/100
- Add demo GIF to README by @MH4GF in https://github.com/MH4GF/tq/pull/99
- Add automated golden rule violation detection (Phase 2) by @MH4GF in https://github.com/MH4GF/tq/pull/101
- Resolve all 30 Rule 11 violations via db.Store test-seam methods by @MH4GF in https://github.com/MH4GF/tq/pull/106
- Add claude_args metadata key for dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/105
- Show available projects in task create help by @MH4GF in https://github.com/MH4GF/tq/pull/103
- Add Homebrew cask distribution via GoReleaser by @MH4GF in https://github.com/MH4GF/tq/pull/107

## [v0.21.11](https://github.com/MH4GF/tq/compare/v0.21.10...v0.21.11) - 2026-04-06
- Improve dispatch prompt and action metadata validation by @MH4GF in https://github.com/MH4GF/tq/pull/74
- Polish TUI: contrast, detail views, interactive slots by @MH4GF in https://github.com/MH4GF/tq/pull/80
- Add --jq flag to all JSON-outputting commands by @MH4GF in https://github.com/MH4GF/tq/pull/75
- Fix noninteractive timeout kills and unnecessary investigate actions by @MH4GF in https://github.com/MH4GF/tq/pull/82
- Fix worktree+plan actions being falsely reaped by noninteractive stale check by @MH4GF in https://github.com/MH4GF/tq/pull/83
- セッションログハートビートによるstale検知 by @MH4GF in https://github.com/MH4GF/tq/pull/84
- Fix tq plugin README documentation drift by @MH4GF in https://github.com/MH4GF/tq/pull/86

## [v0.21.10](https://github.com/MH4GF/tq/compare/v0.21.9...v0.21.10) - 2026-03-31
- Fix tq task create invocation in watch.md by @MH4GF in https://github.com/MH4GF/tq/pull/71
- Redesign TUI with project cards and refined visual hierarchy by @MH4GF in https://github.com/MH4GF/tq/pull/77
- Add noninteractive stale detection to reapStaleActions by @MH4GF in https://github.com/MH4GF/tq/pull/73

## [v0.21.9](https://github.com/MH4GF/tq/compare/v0.21.8...v0.21.9) - 2026-03-30
- Prevent markTerminal from overwriting already-terminal actions by @MH4GF in https://github.com/MH4GF/tq/pull/68
- Show unfocused hint for action create on unfocused projects by @MH4GF in https://github.com/MH4GF/tq/pull/70
- Add --cascade flag to project delete by @MH4GF in https://github.com/MH4GF/tq/pull/72

## [v0.21.8](https://github.com/MH4GF/tq/compare/v0.21.7...v0.21.8) - 2026-03-26
- Add /quality-review command as quality gate by @MH4GF in https://github.com/MH4GF/tq/pull/63
- Add meta key descriptions and fix stale prompt references by @MH4GF in https://github.com/MH4GF/tq/pull/62
- Add draft status check to self-review Phase 4 by @MH4GF in https://github.com/MH4GF/tq/pull/49

## [v0.21.7](https://github.com/MH4GF/tq/compare/v0.21.6...v0.21.7) - 2026-03-26
- Extract gh api calls into standalone scripts for gh-notifications plugin by @MH4GF in https://github.com/MH4GF/tq/pull/65

## [v0.21.6](https://github.com/MH4GF/tq/compare/v0.21.5...v0.21.6) - 2026-03-25
- Move tq dispatch to tq action dispatch subcommand by @MH4GF in https://github.com/MH4GF/tq/pull/58
- Place --worktree flag after prompt in interactive dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/64
- Fix watch skill silently swallowing gh api errors by @MH4GF in https://github.com/MH4GF/tq/pull/60

## [v0.21.6](https://github.com/MH4GF/tq/compare/v0.21.5...v0.21.6) - 2026-03-25
- Move tq dispatch to tq action dispatch subcommand by @MH4GF in https://github.com/MH4GF/tq/pull/58

## [v0.21.5](https://github.com/MH4GF/tq/compare/v0.21.4...v0.21.5) - 2026-03-25
- Add tq action update command by @MH4GF in https://github.com/MH4GF/tq/pull/56
- Show pending counts split by dispatch-enabled projects by @MH4GF in https://github.com/MH4GF/tq/pull/55

## [v0.21.4](https://github.com/MH4GF/tq/compare/v0.21.3...v0.21.4) - 2026-03-24
- Replace schedule auto-disable with UpdateTask guard by @MH4GF in https://github.com/MH4GF/tq/pull/52
- Instruct manager skill to use --jq flag instead of piped commands by @MH4GF in https://github.com/MH4GF/tq/pull/53

## [v0.21.3](https://github.com/MH4GF/tq/compare/v0.21.2...v0.21.3) - 2026-03-23
- Add docs-reviewer sub-agent and fix documentation drift by @MH4GF in https://github.com/MH4GF/tq/pull/48
- Document schedule metadata dispatch keys by @MH4GF in https://github.com/MH4GF/tq/pull/51

## [v0.21.2](https://github.com/MH4GF/tq/compare/v0.21.1...v0.21.2) - 2026-03-23
- Add --worktree flag support for claude CLI dispatch by @MH4GF in https://github.com/MH4GF/tq/pull/37
- Add gh-notifications plugin and instruction-based schedule support by @MH4GF in https://github.com/MH4GF/tq/pull/32
- Add tq search command for cross-entity full-text search by @MH4GF in https://github.com/MH4GF/tq/pull/25
- Expand golangci-lint rules for tech debt detection by @MH4GF in https://github.com/MH4GF/tq/pull/35
- Register gh-notifications plugin in marketplace manifest by @MH4GF in https://github.com/MH4GF/tq/pull/41
- Remove prompt template system, migrate to instruction-only by @MH4GF in https://github.com/MH4GF/tq/pull/38
- Add PR processing commands to gh-notifications plugin by @MH4GF in https://github.com/MH4GF/tq/pull/42
- Bump gh-notifications plugin version to 0.2.0 by @MH4GF in https://github.com/MH4GF/tq/pull/44
- Add common preamble/postamble to dispatched action instructions by @MH4GF in https://github.com/MH4GF/tq/pull/46
- Change action create instruction to positional argument by @MH4GF in https://github.com/MH4GF/tq/pull/47
- Add gh-notifications plugin to tagpr versionFile by @MH4GF in https://github.com/MH4GF/tq/pull/45
- Use tq search for task lookup in skills by @MH4GF in https://github.com/MH4GF/tq/pull/43
- Store and display worker's actual max_interactive by @MH4GF in https://github.com/MH4GF/tq/pull/40

## [v0.21.1](https://github.com/MH4GF/tq/compare/v0.21.0...v0.21.1) - 2026-03-21
- Add depguard rules to enforce layered architecture by @MH4GF in https://github.com/MH4GF/tq/pull/33
- Add get template function for optional metadata keys by @MH4GF in https://github.com/MH4GF/tq/pull/34

## [v0.21.0](https://github.com/MH4GF/tq/commits/v0.21.0) - 2026-03-21
- Refactor navigation: hierarchical tasks→queue view with action counts by @MH4GF in https://github.com/MH4GF/tq/pull/2
- Add action status validation and improve reset/cancel logic by @MH4GF in https://github.com/MH4GF/tq/pull/3
- Add semantic versioning with tagpr by @MH4GF in https://github.com/MH4GF/tq/pull/4
- Add Claude Code GitHub Workflow by @MH4GF in https://github.com/MH4GF/tq/pull/6
- Add gh-setup-hooks SessionStart hook for Claude Code on the Web by @MH4GF in https://github.com/MH4GF/tq/pull/9
- Migrate golangci-lint config to v2 and fix all lint errors by @MH4GF in https://github.com/MH4GF/tq/pull/10
- Auto-create investigation actions when actions fail by @MH4GF in https://github.com/MH4GF/tq/pull/7
- Migrate Task.URL from DB column to metadata JSON by @MH4GF in https://github.com/MH4GF/tq/pull/12
- Replace status string literals with constants by @MH4GF in https://github.com/MH4GF/tq/pull/15
- Show existing action details in duplicate block error by @MH4GF in https://github.com/MH4GF/tq/pull/16
- Fix tagpr release config: sync version, delegate releases to GoReleaser by @MH4GF in https://github.com/MH4GF/tq/pull/11
- Add --meta flag to task update command by @MH4GF in https://github.com/MH4GF/tq/pull/17
- Add action_id to auto-created action log messages by @MH4GF in https://github.com/MH4GF/tq/pull/19
- Add CODEOWNERS with @MH4GF as default owner by @MH4GF in https://github.com/MH4GF/tq/pull/23
- Add structured result format guidance to action done by @MH4GF in https://github.com/MH4GF/tq/pull/24
- Make prompt_id optional: support instruction-only actions by @MH4GF in https://github.com/MH4GF/tq/pull/22
- Add task get and action get commands by @MH4GF in https://github.com/MH4GF/tq/pull/21
- Add queue status display to action create by @MH4GF in https://github.com/MH4GF/tq/pull/20
- Expand golangci-lint config with additional linters and formatters by @MH4GF in https://github.com/MH4GF/tq/pull/27
- Use PAT instead of GITHUB_TOKEN in tagpr workflow by @MH4GF in https://github.com/MH4GF/tq/pull/29
- Add --limit flag to all list commands by @MH4GF in https://github.com/MH4GF/tq/pull/26
- Expand golangci-lint config and apply auto-fixes by @MH4GF in https://github.com/MH4GF/tq/pull/30
- Add --jq flag to list commands by @MH4GF in https://github.com/MH4GF/tq/pull/28

## [v0.21.0](https://github.com/MH4GF/tq/commits/v0.21.0) - 2026-03-21
- Refactor navigation: hierarchical tasks→queue view with action counts by @MH4GF in https://github.com/MH4GF/tq/pull/2
- Add action status validation and improve reset/cancel logic by @MH4GF in https://github.com/MH4GF/tq/pull/3
- Add semantic versioning with tagpr by @MH4GF in https://github.com/MH4GF/tq/pull/4
- Add Claude Code GitHub Workflow by @MH4GF in https://github.com/MH4GF/tq/pull/6
- Add gh-setup-hooks SessionStart hook for Claude Code on the Web by @MH4GF in https://github.com/MH4GF/tq/pull/9
- Migrate golangci-lint config to v2 and fix all lint errors by @MH4GF in https://github.com/MH4GF/tq/pull/10
- Auto-create investigation actions when actions fail by @MH4GF in https://github.com/MH4GF/tq/pull/7
- Migrate Task.URL from DB column to metadata JSON by @MH4GF in https://github.com/MH4GF/tq/pull/12
- Replace status string literals with constants by @MH4GF in https://github.com/MH4GF/tq/pull/15
- Show existing action details in duplicate block error by @MH4GF in https://github.com/MH4GF/tq/pull/16
- Fix tagpr release config: sync version, delegate releases to GoReleaser by @MH4GF in https://github.com/MH4GF/tq/pull/11
- Add --meta flag to task update command by @MH4GF in https://github.com/MH4GF/tq/pull/17
- Add action_id to auto-created action log messages by @MH4GF in https://github.com/MH4GF/tq/pull/19
- Add CODEOWNERS with @MH4GF as default owner by @MH4GF in https://github.com/MH4GF/tq/pull/23
- Add structured result format guidance to action done by @MH4GF in https://github.com/MH4GF/tq/pull/24
- Make prompt_id optional: support instruction-only actions by @MH4GF in https://github.com/MH4GF/tq/pull/22
- Add task get and action get commands by @MH4GF in https://github.com/MH4GF/tq/pull/21
- Add queue status display to action create by @MH4GF in https://github.com/MH4GF/tq/pull/20
- Expand golangci-lint config with additional linters and formatters by @MH4GF in https://github.com/MH4GF/tq/pull/27
