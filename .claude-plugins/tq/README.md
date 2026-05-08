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

## Commands

### `/tq:done <action_id> [summary]`

Mark a tq action as done, then judge task-level completion and propose follow-up actions.

Use this from a Claude Code session launched via the tq interactive worker.

```
/tq:done           # auto-detect action_id, auto-generate summary
/tq:done 42        # specify action_id, auto-generate summary
/tq:done 42 Fix auth bug  # specify action_id and summary
```

### `/tq:failed [action_id]`

Mark a tq action as failed, then judge task-level completion and propose follow-up actions (retry or alternative approach when needed). Use for cases that could not be completed (missing permissions, broken environment, external API outage, CI flake, etc.). Failed actions can be returned to pending with `tq action reset` and retried.

```
/tq:failed           # auto-detect action_id
/tq:failed 42        # specify action_id
```

### `/tq:cancel [action_id]`

Cancel a tq action and record improvement suggestions, then judge task-level completion and propose follow-up actions.

```
/tq:cancel           # auto-detect action_id
/tq:cancel 42        # specify action_id
```

### `/tq:create-action [instruction]`

Create a tq action. The instruction is auto-inferred, or the user can specify it.

### `/tq:triage [project_name]`

Inventory open tasks: review status → propose cleanup → execute.

## CLI commands used

### `tq search <keyword>`

Cross-cutting full-text search over task titles, task metadata, task status-change reasons, action titles, action results, and action metadata. Outputs JSON. Each result includes `project_id`. Filter with `--jq`, or scope to a single project with `--project <ID>`.

```
tq search "login bug"
tq search deploy --project 1
```

## Skills

### `tq:manager`

tqタスク管理者。「タスク作って」「アクション追加して」「完了にして」「状況見せて」「割り込み実行して」「スケジュール実行したい」で発動

`skills/manager/SKILL.md`

## hooks

### `SessionStart`

Runs `tq internal claude-session-record` to record the `session_id` issued by Claude Code into the action metadata's `claude_session_id`.

- The startup env variable `TQ_ACTION_ID` identifies the target action 1:1. Only Claude sessions launched via tq dispatch are recorded.
- No side effects on manual claude launches without `TQ_ACTION_ID` (silent exit).
- Hook failures never disrupt the Claude session (always exits 0).
