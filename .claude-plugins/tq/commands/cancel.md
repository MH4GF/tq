---
description: Cancel a tq action and record improvement suggestions
argument-hint: "<action_id>"
allowed-tools: Bash(tq *)
---

# tq action cancel

IMPORTANT: Run `tq action cancel --help` first to understand the expected reason format.

## Find action_id

1. `$ARGUMENTS` if numeric
2. Environment variable `TQ_ACTION_ID` — always check this first. It is pre-set by the dispatcher and avoids an extra CLI call.
3. Search running actions: `tq action list --status running`
4. If none works, ask the user

## Investigate before cancelling

Review the task's action history to understand why this action was created:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action.

## Execute

`tq action cancel <action_id> '<reason>'`
