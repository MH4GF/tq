# Changelog

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
