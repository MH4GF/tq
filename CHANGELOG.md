# Changelog

## [v0.21.16](https://github.com/MH4GF/tq/compare/v0.21.15...v0.21.16) - 2026-04-26
- Convert IgnoresDispatchSession test to table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/200
- Add PostToolUse hook for auto golangci-lint fmt + lint diag injection by @MH4GF in https://github.com/MH4GF/tq/pull/202

## [v0.21.15](https://github.com/MH4GF/tq/compare/v0.21.14...v0.21.15) - 2026-04-26
- refactor(task)!: cap nested actions in task list/get to latest 10 by @MH4GF in https://github.com/MH4GF/tq/pull/143
- feat(gh-ops): use `gh release view` for Release notification fetch by @MH4GF in https://github.com/MH4GF/tq/pull/145
- docs(tq): clarify create-action skill delegates to separate session by @MH4GF in https://github.com/MH4GF/tq/pull/146
- Add task-level follow-up flow to /tq:done by @MH4GF in https://github.com/MH4GF/tq/pull/147
- docs: add worktree naming guidance for claude_args by @MH4GF in https://github.com/MH4GF/tq/pull/148
- docs(tq): add project focus/unfocus awareness to triage by @MH4GF in https://github.com/MH4GF/tq/pull/150
- fix(dispatch): guard tmux send-keys against oversized instructions by @MH4GF in https://github.com/MH4GF/tq/pull/149
- docs(gh-ops): align README descriptions with command frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/155
- refactor(tui): convert TestTabSwitch to table-driven pattern by @MH4GF in https://github.com/MH4GF/tq/pull/157
- docs: add completion subcommand to cli-reference by @MH4GF in https://github.com/MH4GF/tq/pull/156
- docs(tq): align cancel skill follow-up with done skill pattern by @MH4GF in https://github.com/MH4GF/tq/pull/158
- docs(gh-ops): fix README drift against watch.md by @MH4GF in https://github.com/MH4GF/tq/pull/159
- test(schedule): collapse TestScheduleCreate cluster into table-driven form by @MH4GF in https://github.com/MH4GF/tq/pull/132
- test(done): collapse done_test.go into table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/125
- test(cmd): convert dispatch_test.go to table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/126
- test(dispatch): collapse TestFileSessionLogChecker variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/142
- test(project): collapse TestDeleteProject variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/135
- fix(gh-ops): resolve gh api drift in watch.md by @MH4GF in https://github.com/MH4GF/tq/pull/160
- refactor(cmd): convert TestResolveDBPath to table-driven pattern by @MH4GF in https://github.com/MH4GF/tq/pull/151
- refactor(dispatch): table-driven SkipsTimeout test + use constants by @MH4GF in https://github.com/MH4GF/tq/pull/152
- refactor(db): table-driven tests for ListTasks and MergeTaskMetadata by @MH4GF in https://github.com/MH4GF/tq/pull/153
- extract quality-review state recording into standalone script by @MH4GF in https://github.com/MH4GF/tq/pull/161
- test(investigate_failure): collapse TestCreateInvestigateFailureAction into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/137
- refactor(db): convert TestListActions/TestUpdateAction to table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/154
- Align failed.md with done.md/cancel.md follow-up pattern by @MH4GF in https://github.com/MH4GF/tq/pull/163
- Remove legacy meta keys from cli-reference by @MH4GF in https://github.com/MH4GF/tq/pull/162
- test(permission_block): collapse TestCreatePermissionBlockAction subtests into table-driven form by @MH4GF in https://github.com/MH4GF/tq/pull/141
- Block task completion when pending/running actions exist by @MH4GF in https://github.com/MH4GF/tq/pull/116
- Add quality-review and simplify skill permissions to settings.json by @MH4GF in https://github.com/MH4GF/tq/pull/114
- docs(tq-plugin): align /tq:failed and /tq:cancel docs with behavior by @MH4GF in https://github.com/MH4GF/tq/pull/164
- Add project_id to search results and --project filter by @MH4GF in https://github.com/MH4GF/tq/pull/115
- Add summarize-and-assess-risk step to gh-ops:merge-pr by @MH4GF in https://github.com/MH4GF/tq/pull/165
- docs: add cancelled state to Action State Machine diagram by @MH4GF in https://github.com/MH4GF/tq/pull/166
- Add deadcode CI gate (Rule 13) by @MH4GF in https://github.com/MH4GF/tq/pull/167
- Delete 3 allowlisted dead identifiers (Rule 13) by @MH4GF in https://github.com/MH4GF/tq/pull/169
- Add task notes mechanism for /tq:triage keep judgments by @MH4GF in https://github.com/MH4GF/tq/pull/170
- cmd action done: validate status to avoid silent no-op by @MH4GF in https://github.com/MH4GF/tq/pull/173
- Surface MarkFailed errors in dispatch error paths by @MH4GF in https://github.com/MH4GF/tq/pull/172
- Add diff-scoped E2E execution as third quality-review parallel agent by @MH4GF in https://github.com/MH4GF/tq/pull/171
- Split resolveWorkDir into pure resolver + apply step by @MH4GF in https://github.com/MH4GF/tq/pull/175
- Add Rule 14: forbid *ForTest method calls in production code by @MH4GF in https://github.com/MH4GF/tq/pull/174
- Replace per-task ListActions N+1 with bulk call in TUI loadTasks by @MH4GF in https://github.com/MH4GF/tq/pull/176
- Wrap markTerminal SELECT+UPDATE in a transaction by @MH4GF in https://github.com/MH4GF/tq/pull/178
- Auto-clear transient TUI messages with TTL by @MH4GF in https://github.com/MH4GF/tq/pull/179
- TUI: surface DB errors in loadTasks and dispatch toggle by @MH4GF in https://github.com/MH4GF/tq/pull/177
- Sync action lifecycle docs and help text with dispatched state by @MH4GF in https://github.com/MH4GF/tq/pull/168
- Add task detail view to TUI with status_history and notes by @MH4GF in https://github.com/MH4GF/tq/pull/181
- Sync action/event tables in cli-reference with help output by @MH4GF in https://github.com/MH4GF/tq/pull/182
- fix(claude-review): add --comment to actually post review comments by @MH4GF in https://github.com/MH4GF/tq/pull/180
- respond-review: preserve quoted comments and drop Thread N numbering by @MH4GF in https://github.com/MH4GF/tq/pull/183
- Add /gc-exploratory skill chained from /gc-golden-rules by @MH4GF in https://github.com/MH4GF/tq/pull/184
- Collapse cmd/add_test.go into table-driven TestAdd by @MH4GF in https://github.com/MH4GF/tq/pull/185
- Collapse TestTaskList variants into one table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/187
- Collapse cmd/jq_test.go into a single table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/188
- Refactor TestFilterForOpenTask into table-driven form by @MH4GF in https://github.com/MH4GF/tq/pull/190
- Consolidate dispatch ExecuteAction tests into table-driven form by @MH4GF in https://github.com/MH4GF/tq/pull/191
- Collapse TestActionUpdate variants into table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/189
- Fix CLI reference drift against actual tq --help output by @MH4GF in https://github.com/MH4GF/tq/pull/192
- Add tq action resume to continue claude sessions by @MH4GF in https://github.com/MH4GF/tq/pull/186
- Convert TestUpdateTask_BlockedByActiveActions to table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/193
- Restore tq task note and search --project docs (revert PR #192) by @MH4GF in https://github.com/MH4GF/tq/pull/194
- Add Stop hook to push back on incomplete sessions by @MH4GF in https://github.com/MH4GF/tq/pull/195
- Fix tagpr workflow auth failure on git fetch --unshallow by @MH4GF in https://github.com/MH4GF/tq/pull/197
- Fix Stop hook CI status detection by @MH4GF in https://github.com/MH4GF/tq/pull/196
- Honor --session flag in tq action resume interactive mode by @MH4GF in https://github.com/MH4GF/tq/pull/198

## [v0.21.15](https://github.com/MH4GF/tq/compare/v0.21.14...v0.21.15) - 2026-04-19
- refactor(task)!: cap nested actions in task list/get to latest 10 by @MH4GF in https://github.com/MH4GF/tq/pull/143

## [v0.21.14](https://github.com/MH4GF/tq/compare/v0.21.13...v0.21.14) - 2026-04-19
- test(list): collapse list_test.go cases into table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/128
- test(fail): collapse TestFail variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/127
- Decouple action state transitions from tmux process termination by @MH4GF in https://github.com/MH4GF/tq/pull/130
- test(task): collapse TestTaskCreate variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/133
- test(project): collapse TestProjectCreate variants into table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/131
- test(task): collapse TestTaskUpdate variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/134
- docs(cli-reference): sync with current --help output by @MH4GF in https://github.com/MH4GF/tq/pull/138
- test(dispatch): collapse TestCheckSchedules variants into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/139
- Enforce quality-review before gh pr create via hook by @MH4GF in https://github.com/MH4GF/tq/pull/136
- test(dispatch): collapse TestReapStaleActions_* into table-driven subtests by @MH4GF in https://github.com/MH4GF/tq/pull/140

## [v0.21.13](https://github.com/MH4GF/tq/compare/v0.21.12...v0.21.13) - 2026-04-19
- Add gc-golden-rules periodic GC command (Phase 3) by @MH4GF in https://github.com/MH4GF/tq/pull/109
- quality-review: fix code-simplifier agent type not found error by @MH4GF in https://github.com/MH4GF/tq/pull/111
- Move dispatch context from preamble to postamble by @MH4GF in https://github.com/MH4GF/tq/pull/104
- Replace text help instruction with auto-exec in manager SKILL.md by @MH4GF in https://github.com/MH4GF/tq/pull/112
- gh-ops:watch の gh api /repos 呼び出しを専用サブコマンドに置換 by @MH4GF in https://github.com/MH4GF/tq/pull/113
- Include CLI output in noninteractive worker error messages by @MH4GF in https://github.com/MH4GF/tq/pull/117
- Fix noninteractive actions not saving claude_session_id by @MH4GF in https://github.com/MH4GF/tq/pull/102
- Add work_dir auto-recovery for stale worktree paths by @MH4GF in https://github.com/MH4GF/tq/pull/118
- Add project consistency check step to triage command by @MH4GF in https://github.com/MH4GF/tq/pull/97
- Record task status transition history with required --note by @MH4GF in https://github.com/MH4GF/tq/pull/120
- feat(schedule): support claude_args metadata key by @MH4GF in https://github.com/MH4GF/tq/pull/121
- Search task status change reasons via events payload by @MH4GF in https://github.com/MH4GF/tq/pull/122
- feat: drop permission_mode/worktree metadata in favor of claude_args by @MH4GF in https://github.com/MH4GF/tq/pull/123
- Rewrite tq:triage for lightweight collection and actionable proposals by @MH4GF in https://github.com/MH4GF/tq/pull/124

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
