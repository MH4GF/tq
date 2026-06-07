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

### Standalone via `npx skills`

The skills are also published via the [Agent Skills](https://agentskills.io) ecosystem and can be installed without the Claude Code plugin layer using [vercel-labs/skills](https://github.com/vercel-labs/skills):

```bash
# List the 8 tq skills exposed by this repo
npx skills add MH4GF/tq --list

# Install a specific skill into a target agent
npx skills add MH4GF/tq --skill tq-create-action -a claude-code

# Install all tq skills
npx skills add MH4GF/tq --skill '*' -a claude-code
```

Skill names are prefixed with `tq-` (e.g. `tq-create-action`) to avoid collisions with skills from other publishers when multiple sources are mixed in one agent.

### Standalone via `gh skill` (preview)

[`gh skill`](https://cli.github.com/manual/gh_skill) follows the same Agent Skills spec:

```bash
gh skill install MH4GF/tq tq-create-action
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

### `tq:dep-triage`

Rescue tq actions stuck pending forever because a completion dependency ended failed or cancelled. A blocked action is only released when every blocker reaches a successful terminal state, so a failed/cancelled blocker strands the dependent indefinitely. Use this whenever asked to triage blocked/stuck/stalled pending actions, find actions waiting on a dead dependency, audit blocked_by chains, or unblock the dispatch queue — including the scheduled "/tq:dep-triage" run.

`skills/dep-triage/SKILL.md`

### `tq:manager`

Manage tq tasks and actions on behalf of the user via the tq CLI. Use when the user wants to create a task, add or dispatch an action, check queue status, run something now or interrupt, or schedule a recurring action. Lightweight interactive hub; hands off to tq:create-action / tq:done / tq:failed / tq:cancel / tq:triage for disciplined flows.

`skills/manager/SKILL.md`

Local actions (`mode=interactive` or `noninteractive`) launch via `claude --bg` and differ only in which slot pool they consume. `mode=remote` is a separate cloud-execution path that runs `claude --remote` instead. See [Best practices — Dispatch mode selection](../../docs/best-practices.md#dispatch-mode-selection) for the full decision rule.

### `tq:investigate-incidents`

Cross-event diagnosis of recent failed actions and permission-blocked actions in the tq queue. Use when the user wants to investigate incidents, summarize recent failures, or review permission-block trends across the queue. Clusters incidents and cross-checks prior remediations rather than per-event firefighting.

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
