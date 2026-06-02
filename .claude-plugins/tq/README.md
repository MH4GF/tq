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

tq task manager. Triggers on natural-language requests like "create a task", "add an action", "show status", "run an interrupt", or "schedule something".

`skills/manager/SKILL.md`

All local actions launch via `claude --bg` regardless of `mode`; the `mode` field only picks a slot pool (`interactive` vs `noninteractive`). See [Best practices — Dispatch mode selection](../../docs/best-practices.md#dispatch-mode-selection) for the full decision rule.

### `tq:investigate-incidents`

Cross-cuts diagnosis of failed actions and permission denials accumulated in the tq queue. Triggers on phrases like "investigate incidents", "summarize recent failures", "show permission block trends", or `/tq:investigate-incidents`.

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
- Local actions (`mode=interactive` or `noninteractive`) launch through `claude --bg`, which does not propagate `TQ_ACTION_ID` into the daemonised session, so this hook is a no-op for them. The queue worker's bg reaper instead polls `~/.claude/jobs/<short>/state.json` each tick and back-fills `claude_session_id` from its `sessionId` field. Effectively, this hook records `claude_session_id` only for `mode=remote` actions.
- Hook failures never disrupt the Claude session (always exits 0).
