# Changelog

## [v0.21.35](https://github.com/MH4GF/tq/compare/v0.21.34...v0.21.35) - 2026-06-10

- build(lint): pin golangci-lint to v2.12.2 (mise + CI) by @MH4GF in https://github.com/MH4GF/tq/pull/412
- feat(action): add --instruction-file flag to tq action create by @MH4GF in https://github.com/MH4GF/tq/pull/414

## [v0.21.34](https://github.com/MH4GF/tq/compare/v0.21.33...v0.21.34) - 2026-06-09

- test(tui): collapse schedules_test.go error tests into table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/374
- feat(tq plugin): expose 7 skills via Agent Skills marketplace for npx skills and gh skill by @MH4GF in https://github.com/MH4GF/tq/pull/376
- docs(CLAUDE.md): enumerate tq search fields to match CLI help by @MH4GF in https://github.com/MH4GF/tq/pull/377
- docs(golden-rules): align per-layer quality grades table with legend by @MH4GF in https://github.com/MH4GF/tq/pull/378
- docs(gh-ops): use canonical plugin CLI flow for installation by @MH4GF in https://github.com/MH4GF/tq/pull/379
- refactor(tui): read claude_session_id via metaKeyClaudeSessionID constant by @MH4GF in https://github.com/MH4GF/tq/pull/380
- docs(golden-rules): correct Rule 17 violation counts to 16 across 8 sites by @MH4GF in https://github.com/MH4GF/tq/pull/383
- test(tui): table-drive SortOrder and ActionStats assertions by @MH4GF in https://github.com/MH4GF/tq/pull/382
- fix(db): wrap MergeActionMetadata SELECT+merge+UPDATE in a transaction by @MH4GF in https://github.com/MH4GF/tq/pull/381
- docs: distribute tq-dep-triage skill via plugin by @MH4GF in https://github.com/MH4GF/tq/pull/384
- fix(db): wrap UpdateAction read+validate+write in a tx by @MH4GF in https://github.com/MH4GF/tq/pull/385
- test(tui): collapse TestTasksModel_Dispatch* trio into table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/387
- docs(tq): mark /tq:done action_id as optional in argument-hint by @MH4GF in https://github.com/MH4GF/tq/pull/386
- docs(cli-reference): align command-table descriptions with CLI Short strings by @MH4GF in https://github.com/MH4GF/tq/pull/388
- fix(dispatch): probe next pending action when slot pool defers head by @MH4GF in https://github.com/MH4GF/tq/pull/391
- docs(cmd): add executor to --meta help on action update + schedule create/update by @MH4GF in https://github.com/MH4GF/tq/pull/392
- fix(tui): auto-clear SchedulesModel toast messages after TTL by @MH4GF in https://github.com/MH4GF/tq/pull/389
- fix(cmd/add): atomically create action and dependency edges by @MH4GF in https://github.com/MH4GF/tq/pull/393
- fix(dispatch): defer admission callback errors instead of MarkFailed by @MH4GF in https://github.com/MH4GF/tq/pull/390
- fix(cmd/schedule): reject create/update --task on terminal task by @MH4GF in https://github.com/MH4GF/tq/pull/394
- perf(tui): load task detail asynchronously via tea.Cmd by @MH4GF in https://github.com/MH4GF/tq/pull/396
- fix(dispatch): atomic schedule-tick prevents duplicate actions by @MH4GF in https://github.com/MH4GF/tq/pull/395
- test(dispatch): cover bg merge-failure → reapOrphans path end-to-end by @MH4GF in https://github.com/MH4GF/tq/pull/397
- test(db): refactor TestHasActiveActionsForSchedules to table-driven by @MH4GF in https://github.com/MH4GF/tq/pull/399
- refactor(dispatch): table-drive TestReapBg_*/TestReapOrphans_* family by @MH4GF in https://github.com/MH4GF/tq/pull/398
- security(ci): pin third-party actions in claude.yml to commit SHAs by @MH4GF in https://github.com/MH4GF/tq/pull/400
- fix(db): preserve worker result when MarkDone loses race to terminal transition by @MH4GF in https://github.com/MH4GF/tq/pull/402
- docs(skill): drop unused context: fork from turso-query-watch frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/403
- docs(plugin): fix manager skill mode guidance for current dispatch reality by @MH4GF in https://github.com/MH4GF/tq/pull/401
- docs(plugins/tq): fix wrong --meta dispatch_after instruction in failed skill by @MH4GF in https://github.com/MH4GF/tq/pull/404
- refactor(dispatch): route remote_session key through MetaKey constant + close Rule 10 writer-side gap by @MH4GF in https://github.com/MH4GF/tq/pull/405
- docs(triage): always exclude recurring tasks from Step 6 AskUserQuestion by @MH4GF in https://github.com/MH4GF/tq/pull/406
- refactor(gh-ops/respond-review): drop pre-emptive "Will address" step by @MH4GF in https://github.com/MH4GF/tq/pull/407
- feat(db): absorb SQLite database-locked errors via DSN PRAGMA + lock retry by @MH4GF in https://github.com/MH4GF/tq/pull/408
- fix(db): replace action dependencies atomically by @MH4GF in https://github.com/MH4GF/tq/pull/409
- fix(dispatch): reject tq mode names as --permission-mode value in claude_args by @MH4GF in https://github.com/MH4GF/tq/pull/410
- fix(db): close remaining task-attach guard gaps (schedule enable, action update, TUI toggle) by @MH4GF in https://github.com/MH4GF/tq/pull/411

## [v0.21.33](https://github.com/MH4GF/tq/compare/v0.21.32...v0.21.33) - 2026-06-04

- docs(plugin-readme): refresh SessionStart and dispatch-mode notes after unified bg path by @MH4GF in https://github.com/MH4GF/tq/pull/370
- feat(triage): PR-merge override for skip rule by @MH4GF in https://github.com/MH4GF/tq/pull/371
- fix(dispatch): apply bg hard timeout only to noninteractive actions by @MH4GF in https://github.com/MH4GF/tq/pull/373

## [v0.21.32](https://github.com/MH4GF/tq/compare/v0.21.31...v0.21.32) - 2026-06-01
- refactor: unify action launch on claude --bg, retire experimental_bg by @MH4GF in https://github.com/MH4GF/tq/pull/367
- fix(db): guard experimental_bg migration against malformed metadata rows by @MH4GF in https://github.com/MH4GF/tq/pull/369

## [v0.21.31](https://github.com/MH4GF/tq/compare/v0.21.30...v0.21.31) - 2026-06-01
- feat: auto-record claude_session_id for experimental_bg; allow --meta on done by @MH4GF in https://github.com/MH4GF/tq/pull/363
- Migrate tq plugin slash commands to Agent Skills by @MH4GF in https://github.com/MH4GF/tq/pull/365
- fix: respond-review draft template passes the unslop guard by @MH4GF in https://github.com/MH4GF/tq/pull/366

## [v0.21.30](https://github.com/MH4GF/tq/compare/v0.21.29...v0.21.30) - 2026-05-23
- feat(gh-ops): add Edit plan field to respond-review draft by @MH4GF in https://github.com/MH4GF/tq/pull/355
- feat(tui): show full action context in detail view by @MH4GF in https://github.com/MH4GF/tq/pull/357
- docs(cli-reference): document r/f TUI shortcuts alongside d by @MH4GF in https://github.com/MH4GF/tq/pull/358
- fix(dispatch): retry MarkDone and emit explicit FAILED on persistent write error by @MH4GF in https://github.com/MH4GF/tq/pull/359
- fix(dispatch): mark action FAILED when DeferToPending itself fails by @MH4GF in https://github.com/MH4GF/tq/pull/360
- fix(dispatch): use HeartbeatFreshness in early-stale watchdog to stop leaking running actions by @MH4GF in https://github.com/MH4GF/tq/pull/361
- rename simplify → code-review in /quality-review by @MH4GF in https://github.com/MH4GF/tq/pull/362

## [v0.21.29](https://github.com/MH4GF/tq/compare/v0.21.28...v0.21.29) - 2026-05-18
- fix(dispatch): stop reaping live interactive actions via tmux window check by @MH4GF in https://github.com/MH4GF/tq/pull/353

## [v0.21.28](https://github.com/MH4GF/tq/compare/v0.21.27...v0.21.28) - 2026-05-18
- docs: add best-practices guide with dispatch mode selection by @MH4GF in https://github.com/MH4GF/tq/pull/351

## [v0.21.27](https://github.com/MH4GF/tq/compare/v0.21.26...v0.21.27) - 2026-05-18
- docs: reconcile tq search description with CLI help by @MH4GF in https://github.com/MH4GF/tq/pull/342
- docs: reconcile action subcommand table with live CLI help by @MH4GF in https://github.com/MH4GF/tq/pull/343
- test(db): lock SQL/Go dependency-satisfaction parity by @MH4GF in https://github.com/MH4GF/tq/pull/345
- fix(dispatch): strip ANSI before parsing claude --bg short id by @MH4GF in https://github.com/MH4GF/tq/pull/347
- Make tq task update atomic via single UpdateTaskFields transaction by @MH4GF in https://github.com/MH4GF/tq/pull/346
- feat(config): DB-backed global default dispatch mode by @MH4GF in https://github.com/MH4GF/tq/pull/348
- fix: exclude false positives in investigate-incidents denial query by @MH4GF in https://github.com/MH4GF/tq/pull/349
- feat(tui): add 'd' keybinding to dispatch a pending action by @MH4GF in https://github.com/MH4GF/tq/pull/350

## [v0.21.26](https://github.com/MH4GF/tq/compare/v0.21.25...v0.21.26) - 2026-05-18
- db: remove dead ListTasksByStatus (Rule 19 burn-down) by @MH4GF in https://github.com/MH4GF/tq/pull/335
- fix(db): purge dangling action_dependencies on task/action delete by @MH4GF in https://github.com/MH4GF/tq/pull/341

## [v0.21.25](https://github.com/MH4GF/tq/compare/v0.21.24...v0.21.25) - 2026-05-16
- refactor: remove dead db.Store.UpdateScheduleRun by @MH4GF in https://github.com/MH4GF/tq/pull/336
- noninteractive: attach raw output on JSON parse failure by @MH4GF in https://github.com/MH4GF/tq/pull/339

## [v0.21.24](https://github.com/MH4GF/tq/compare/v0.21.23...v0.21.24) - 2026-05-16
- docs(create-action): rewrite Build instruction around goal-state + value by @MH4GF in https://github.com/MH4GF/tq/pull/318
- dispatch: couple schedule insert spec with its success run update by @MH4GF in https://github.com/MH4GF/tq/pull/320
- docs(tq-plugin): make worker model reference version-agnostic in create-action by @MH4GF in https://github.com/MH4GF/tq/pull/321
- gc-exploratory: dedup findings against this task's action history by @MH4GF in https://github.com/MH4GF/tq/pull/322
- feat(triage): add recurring-task exclusion rule to /tq:triage by @MH4GF in https://github.com/MH4GF/tq/pull/323
- tq:done: gate done on a merged PR for code-change actions by @MH4GF in https://github.com/MH4GF/tq/pull/324
- db: include failure reason in BulkMarkFailed event payload by @MH4GF in https://github.com/MH4GF/tq/pull/325
- db: remove dead EnsureNotificationsProject chain (Rule 19 burn-down) by @MH4GF in https://github.com/MH4GF/tq/pull/326
- feat: completion-dependency dispatch gating for actions by @MH4GF in https://github.com/MH4GF/tq/pull/327
- db: remove dead Store.IsWorkerRunning method (Rule 19 burn-down) by @MH4GF in https://github.com/MH4GF/tq/pull/329
- db: remove dead HasActiveActionWithMeta (Rule 19 burn-down) by @MH4GF in https://github.com/MH4GF/tq/pull/330
- refactor: remove dead db.Store.SetAllDispatchEnabled (Rule 19 burn-down) by @MH4GF in https://github.com/MH4GF/tq/pull/331
- feat(cmd): add tq schedule get subcommand (Rule 19 burn-down: GetSchedule) by @MH4GF in https://github.com/MH4GF/tq/pull/332
- cmd: clarify --max-interactive help text covers shared pool by @MH4GF in https://github.com/MH4GF/tq/pull/333
- Rule 19 burn-down: delete dead db.Store.GetOrCreateTriageTask + EnsureTask by @MH4GF in https://github.com/MH4GF/tq/pull/328
- test(dispatch): table-drive CheckSchedules metadata-failure tests by @MH4GF in https://github.com/MH4GF/tq/pull/334
- fix(dispatch): don't misclassify completed long-running actions as early-stale by @MH4GF in https://github.com/MH4GF/tq/pull/337

## [v0.21.23](https://github.com/MH4GF/tq/compare/v0.21.22...v0.21.23) - 2026-05-16
- db: trigger-maintained task_action_counts + Rule 18 (no aggregates in hot paths) by @MH4GF in https://github.com/MH4GF/tq/pull/277
- accept multiple thread_ids in gh-mark-notification-read by @MH4GF in https://github.com/MH4GF/tq/pull/280
- golden-rules: add Rule 15 (no db.Store calls in for-range loops) by @MH4GF in https://github.com/MH4GF/tq/pull/281
- dispatch: phase 2 — remove polling-based claude_session_id capture by @MH4GF in https://github.com/MH4GF/tq/pull/282
- feat(action): support per-action --work-dir override by @MH4GF in https://github.com/MH4GF/tq/pull/283
- dispatch: add experimental_bg mode (research preview) by @MH4GF in https://github.com/MH4GF/tq/pull/284
- dispatch: defer slot-capped actions with dispatch_after backoff and fix TUI slot gauge by @MH4GF in https://github.com/MH4GF/tq/pull/285
- dispatch: retire per-event investigate/permission-block auto-gen, route to /tq:investigate-incidents skill by @MH4GF in https://github.com/MH4GF/tq/pull/286
- tui: open action detail for any status (instruction + result) by @MH4GF in https://github.com/MH4GF/tq/pull/288
- docs(tq-plugin): sync README command descriptions to frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/289
- dispatch: mark action FAILED on async goroutine panic by @MH4GF in https://github.com/MH4GF/tq/pull/290
- tui(schedules): surface ListSchedules error instead of silent empty list by @MH4GF in https://github.com/MH4GF/tq/pull/291
- db: add idx_schedules_task to eliminate 3 Rule 17 SCANs by @MH4GF in https://github.com/MH4GF/tq/pull/292
- Add weekly Turso rows-read regression watch by @MH4GF in https://github.com/MH4GF/tq/pull/293
- goldenrules: add Rule 19 detecting test-only-reachable db.Store methods by @MH4GF in https://github.com/MH4GF/tq/pull/294
- docs(rule17): annotate db/event.go:69 as LIMIT-bounded PK scan by @MH4GF in https://github.com/MH4GF/tq/pull/296
- docs(tq-plugin): align /tq:done /tq:cancel /tq:triage README with frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/295
- dispatch: record last_error on schedule metadata parse failure by @MH4GF in https://github.com/MH4GF/tq/pull/297
- tui: surface schedule toggle/delete DB errors instead of silently swallowing by @MH4GF in https://github.com/MH4GF/tq/pull/298
- docs: align tq task list help with docs (latest 10 nested actions) by @MH4GF in https://github.com/MH4GF/tq/pull/299
- docs(CLAUDE.md): list top-level tq subcommands by @MH4GF in https://github.com/MH4GF/tq/pull/253
- Add context: fork to investigate-incidents skill by @MH4GF in https://github.com/MH4GF/tq/pull/287
- test: collapse MultipleStale reap tests into table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/301
- docs(tq-plugin): match /tq:failed README desc to frontmatter verbatim by @MH4GF in https://github.com/MH4GF/tq/pull/303
- docs: surface executor --meta key in tq action create help by @MH4GF in https://github.com/MH4GF/tq/pull/304
- db: delete legacy claude-flags migration to burn down Rule 16 by @MH4GF in https://github.com/MH4GF/tq/pull/305
- cmd/action: include recovery hint in done error message by @MH4GF in https://github.com/MH4GF/tq/pull/306
- test(tui): table-drive TaskDetailView_WithNotes substring assertions by @MH4GF in https://github.com/MH4GF/tq/pull/307
- docs(tq-plugin): sync /tq:triage README desc to frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/308
- perf(dispatch): skip reaper prefetch when no action is reapable by @MH4GF in https://github.com/MH4GF/tq/pull/309
- docs(skill): use bold instead of h2 in create-action instruction examples by @MH4GF in https://github.com/MH4GF/tq/pull/310
- fix(goldenrules): check scanner/path errors so violations are not silently skipped by @MH4GF in https://github.com/MH4GF/tq/pull/311
- docs(tq:done): require follow-up action for remaining entries by @MH4GF in https://github.com/MH4GF/tq/pull/312
- docs(tq-plugin): sync tq search README desc to CLI help wording by @MH4GF in https://github.com/MH4GF/tq/pull/313
- fix(db): normalize schedule_id to INTEGER in HasActiveActionsForSchedules by @MH4GF in https://github.com/MH4GF/tq/pull/314
- fix(tui): keep cursor off decorative lines at list edges by @MH4GF in https://github.com/MH4GF/tq/pull/316
- docs(readme): align action state machine diagram with done/fail semantics by @MH4GF in https://github.com/MH4GF/tq/pull/315
- docs(claude): unify architecture-layer arrow notation with golden-rules by @MH4GF in https://github.com/MH4GF/tq/pull/317
- db: convert db.Search to FTS5 trigram, burn Rule 16 ceiling 7→1 by @MH4GF in https://github.com/MH4GF/tq/pull/300

## [v0.21.22](https://github.com/MH4GF/tq/compare/v0.21.21...v0.21.22) - 2026-05-09
- Cut Turso rows-read by date-aware action view + bulk task fetch + 30s tick by @MH4GF in https://github.com/MH4GF/tq/pull/261
- fix(cmd): print 'queue: status unavailable' on DB error in printQueueStatus by @MH4GF in https://github.com/MH4GF/tq/pull/263
- Normalize doc language: convert mixed-Japanese to English by @MH4GF in https://github.com/MH4GF/tq/pull/264
- plugin: mark create-action instruction argument as optional by @MH4GF in https://github.com/MH4GF/tq/pull/265
- dispatch: extend EarlyDispatchTimeout default 60s → 5m by @MH4GF in https://github.com/MH4GF/tq/pull/266
- db: drop dead ListTasksByProject (Rule 13 deadcode-check blind spot) by @MH4GF in https://github.com/MH4GF/tq/pull/267
- dispatch: exempt cloud-executed actions from reaper via metadata.executor by @MH4GF in https://github.com/MH4GF/tq/pull/270
- fix(gh-ops/watch): replace ## headers in co-review template to avoid bash deny by @MH4GF in https://github.com/MH4GF/tq/pull/271
- cmd: align task get help with docs (latest 10 nested actions) by @MH4GF in https://github.com/MH4GF/tq/pull/269
- plugin: add allowed-tools to triage command frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/268
- db: block task close while a dispatched action is in flight by @MH4GF in https://github.com/MH4GF/tq/pull/272
- claude: enable auto mode + share tq-specific permissions by @MH4GF in https://github.com/MH4GF/tq/pull/274
- fix(quality-review): serialize state ledger writes with flock by @MH4GF in https://github.com/MH4GF/tq/pull/273
- goldenrules: add Rule 16 banning leading-wildcard LIKE in SQL by @MH4GF in https://github.com/MH4GF/tq/pull/275
- add Rule 17: detect EXPLAIN QUERY PLAN SCAN in db/ via golden-rule test by @MH4GF in https://github.com/MH4GF/tq/pull/276

## [v0.21.21](https://github.com/MH4GF/tq/compare/v0.21.20...v0.21.21) - 2026-05-07
- Drop stale interactive validator help text by @MH4GF in https://github.com/MH4GF/tq/pull/245
- Declare tq plugin marketplace + SessionStart auto-install hook by @MH4GF in https://github.com/MH4GF/tq/pull/247
- Add Claude session-log fallback recipe to triage and manager by @MH4GF in https://github.com/MH4GF/tq/pull/248
- Drop isResume parameter from RenderPrompt by @MH4GF in https://github.com/MH4GF/tq/pull/249
- Simplify tq installation command in Claude settings by @MH4GF in https://github.com/MH4GF/tq/pull/250
- gh-ops: add /gh-ops:wait-pr-checks command by @MH4GF in https://github.com/MH4GF/tq/pull/251
- attach: require both tmux_session and tmux_window by @MH4GF in https://github.com/MH4GF/tq/pull/252
- plugins: drop stale multi-line→noninteractive guidance by @MH4GF in https://github.com/MH4GF/tq/pull/254
- docs(tq plugin README): add path reference to skills section by @MH4GF in https://github.com/MH4GF/tq/pull/255
- Surface permanent schedule failures via last_error by @MH4GF in https://github.com/MH4GF/tq/pull/256
- docs(cli-reference): mention status/task filtering on action list row by @MH4GF in https://github.com/MH4GF/tq/pull/257
- Run noninteractive dispatch in goroutines (non-blocking loop, +max-noninteractive cap) by @MH4GF in https://github.com/MH4GF/tq/pull/258
- Emit schedule.updated event from UpdateSchedule by @MH4GF in https://github.com/MH4GF/tq/pull/260
- Emit project.work_dir_changed event in SetWorkDir by @MH4GF in https://github.com/MH4GF/tq/pull/259

## [v0.21.20](https://github.com/MH4GF/tq/compare/v0.21.19...v0.21.20) - 2026-05-06
- Rename session_id to tmux_session/tmux_window + claude side, with migration by @MH4GF in https://github.com/MH4GF/tq/pull/235
- Capture claude_session_id via SessionStart hook by @MH4GF in https://github.com/MH4GF/tq/pull/237
- Allow newlines in interactive instruction validator by @MH4GF in https://github.com/MH4GF/tq/pull/239
- Harden interactive reaper against session_id leaks + early dispatch watchdog by @MH4GF in https://github.com/MH4GF/tq/pull/238
- Add E2E test framework with testscript by @MH4GF in https://github.com/MH4GF/tq/pull/240
- Reject action create under done/archived parent task by @MH4GF in https://github.com/MH4GF/tq/pull/241
- Expand E2E scenarios for action lifecycle, schedule, errors, db precedence, attach by @MH4GF in https://github.com/MH4GF/tq/pull/242
- Add tq action prompt subcommand to bypass tmux send-keys MAX_CANON by @MH4GF in https://github.com/MH4GF/tq/pull/243
- Support libsql:// remote DB via TQ_DB_URL by @MH4GF in https://github.com/MH4GF/tq/pull/244

## [v0.21.19](https://github.com/MH4GF/tq/compare/v0.21.18...v0.21.19) - 2026-05-02
- Merge investigate-failure skip tests into one table-driven test by @MH4GF in https://github.com/MH4GF/tq/pull/229
- Reject Claude permission-mode values as tq action mode by @MH4GF in https://github.com/MH4GF/tq/pull/227
- Stop hook: marker-based loop guard for in-session push-back chain by @MH4GF in https://github.com/MH4GF/tq/pull/228
- Expand create-action skill with Opus 4.7 instruction guidelines by @MH4GF in https://github.com/MH4GF/tq/pull/231
- chore: remove Claude Code Review workflow by @MH4GF in https://github.com/MH4GF/tq/pull/233
- Sync gh-ops plugin version to tagpr release rail by @MH4GF in https://github.com/MH4GF/tq/pull/232
- Make noninteractive Execute heartbeat-aware by @MH4GF in https://github.com/MH4GF/tq/pull/234

## [v0.21.18](https://github.com/MH4GF/tq/compare/v0.21.17...v0.21.18) - 2026-05-02
- Force latest_triage_note checkpoints in tq:triage by @MH4GF in https://github.com/MH4GF/tq/pull/214
- docs: match resume description to --help output by @MH4GF in https://github.com/MH4GF/tq/pull/216
- Reject control characters in interactive instructions by @MH4GF in https://github.com/MH4GF/tq/pull/217
- Update last_run_at on schedule marshal failure to throttle retries by @MH4GF in https://github.com/MH4GF/tq/pull/218
- Reap interactive actions when tmux is unavailable past hard timeout by @MH4GF in https://github.com/MH4GF/tq/pull/219
- Drop nonexistent --until flag from triage Step 7 snooze instructions by @MH4GF in https://github.com/MH4GF/tq/pull/220
- Document interactive-mode control byte rejection by @MH4GF in https://github.com/MH4GF/tq/pull/221
- docs: align tq task list table description with --help output by @MH4GF in https://github.com/MH4GF/tq/pull/222
- Align golden-rules GC prose with [enforced] taxonomy by @MH4GF in https://github.com/MH4GF/tq/pull/223
- Persist session_info on tq action dispatch interactive path by @MH4GF in https://github.com/MH4GF/tq/pull/224
- Surface DB read errors in TUI task detail open by @MH4GF in https://github.com/MH4GF/tq/pull/225
- Document tq action get/attach/reset in CLI reference by @MH4GF in https://github.com/MH4GF/tq/pull/226

## [v0.21.17](https://github.com/MH4GF/tq/compare/v0.21.16...v0.21.17) - 2026-04-30
- Replace /gh-ops:review-pr with /gh-ops:brief-pr by @MH4GF in https://github.com/MH4GF/tq/pull/203
- Add Co-review template + skip merged/closed PRs in /gh-ops:watch by @MH4GF in https://github.com/MH4GF/tq/pull/205
- Restructure dispatch postamble for clearer prompt engineering by @MH4GF in https://github.com/MH4GF/tq/pull/206
- Restructure brief-pr digest for scannability by @MH4GF in https://github.com/MH4GF/tq/pull/207
- Restructure /tq:triage Step 6 to Rumelt's kernel of strategy by @MH4GF in https://github.com/MH4GF/tq/pull/208
- Ignore .claude/scheduled_tasks.lock runtime lock by @MH4GF in https://github.com/MH4GF/tq/pull/209
- Add Auto-mode boundaries to gh-ops:watch skill by @MH4GF in https://github.com/MH4GF/tq/pull/210
- Add task status change reasons to tq search field list by @MH4GF in https://github.com/MH4GF/tq/pull/211
- Sync gh-ops README brief-pr description with command frontmatter by @MH4GF in https://github.com/MH4GF/tq/pull/212
- Evaluate cron schedules in now's timezone by @MH4GF in https://github.com/MH4GF/tq/pull/213

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
