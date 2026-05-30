# tq Claude Code Plugin

Claude Code plugin for operating the tq task queue.

## Installation

### Add as a marketplace source

```bash
claude plugin marketplace add MH4GF/tq
```

### Install the plugin

```bash
claude plugin install tq@tq-marketplace
```

## Skills

All tq operations are packaged as Agent Skills. Each one triggers from natural
language **and** from the matching slash invocation `/tq:<name>` — they are
equivalent. The slash form is still the explicit entry point used by the
dispatch worker prompt and by other skills; natural language lets the same flow
fire without remembering a command name. Each description below is reproduced
verbatim from the skill's `SKILL.md` frontmatter.

### `tq:create-action` — `/tq:create-action [instruction]`

Create a tq action (auto-infer instruction or let user specify)

`skills/create-action/SKILL.md`

### `tq:done` — `/tq:done <action_id> [summary]`

Mark a tq action as done, then judge task-level completion and propose follow-up actions when work remains

`skills/done/SKILL.md`

```
/tq:done           # auto-detect action_id, auto-generate summary
/tq:done 42        # specify action_id, auto-generate summary
/tq:done 42 Fix auth bug  # specify action_id and summary
```

### `tq:failed` — `/tq:failed [action_id]`

Mark a tq action as failed, then judge task-level completion and propose follow-up actions when retry or alternative approach is needed

`skills/failed/SKILL.md`

Use for cases that could not be completed (missing permissions, broken environment, external API outage, CI flake, etc.). Failed actions can be returned to pending with `tq action reset` and retried.

```
/tq:failed           # auto-detect action_id
/tq:failed 42        # specify action_id
```

### `tq:cancel` — `/tq:cancel [action_id]`

Cancel a tq action with improvement suggestions, then judge task-level completion and propose follow-up actions when work remains

`skills/cancel/SKILL.md`

```
/tq:cancel           # auto-detect action_id
/tq:cancel 42        # specify action_id
```

### `tq:triage` — `/tq:triage [project_name]`

Inventory and organize open tasks - review status, propose cleanup, execute

`skills/triage/SKILL.md`

### `tq:manager`

tqタスク管理者。「タスク作って」「アクション追加して」「状況見せて」「割り込み実行して」「スケジュール実行したい」で発動

`skills/manager/SKILL.md`

When creating actions/schedules, prefer the `experimental_bg` or `interactive` dispatch mode and avoid `noninteractive` (it draws from the capped Agent SDK credit / API billing). See [Best practices — Dispatch mode selection](../../docs/best-practices.md#dispatch-mode-selection) for the full decision rule.

### `tq:investigate-incidents`

tqキューに溜まった失敗actionとpermission denialを横断的に診断する。「インシデント調べて」「最近の失敗まとめて」「permission blockの傾向見せて」「/tq:investigate-incidents」で発動

`skills/investigate-incidents/SKILL.md`

Replaces the per-event auto-generated follow-up actions that previously fired on every failure and every permission denial. Recommended usage is a daily schedule:

```bash
tq schedule create --instruction '/tq:investigate-incidents' --task <task_id> --cron '0 9 * * *' \
    --title 'Daily incident review'
```

Run `tq schedule create --help` for available flags.

## CLI commands used

### `tq search <keyword>`

Full-text search across task titles, task metadata, task status change reasons, action titles, action results, and action metadata. Output is JSON. Each result includes `project_id`. Filter with `--jq`, or scope to a single project with `--project <ID>`.

```
tq search "login bug"
tq search deploy --project 1
```

## hooks

### `SessionStart`

Runs `tq internal claude-session-record` to record the `session_id` issued by Claude Code into the action metadata's `claude_session_id`.

- The startup env variable `TQ_ACTION_ID` identifies the target action 1:1. Only Claude sessions launched via tq dispatch are recorded.
- No side effects on manual claude launches without `TQ_ACTION_ID` (silent exit).
- When `CLAUDE_CODE_REMOTE=true` (Claude Code on the web / Cloud Routines) is set, `executor=cloud` is also recorded into metadata. The reaper uses this value to unconditionally skip cloud-executed actions (local session log liveness checks do not apply to them).
- Hook failures never disrupt the Claude session (always exits 0).
