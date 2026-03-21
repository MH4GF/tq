# Changelog

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
