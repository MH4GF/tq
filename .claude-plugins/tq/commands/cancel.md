---
description: Cancel a tq action and record improvement suggestions
argument-hint: "<action_id>"
allowed-tools: Bash(tq *)
---

# tq action cancel

IMPORTANT: Run !`tq action cancel --help` first to understand the expected reason format.

## Find action_id

1. `$ARGUMENTS` if numeric
2. Environment variable `TQ_ACTION_ID` — always check this first. It is pre-set by the dispatcher and avoids an extra CLI call.
3. Search running actions: `tq action list --status running`
4. If none works, ask the user

## Investigate before cancelling

Review the task's action history to understand why this action was created:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action.

## Next Action

Before cancelling, determine if follow-up work is needed:

1. Run `tq action list --task <task_id>` to review action history
2. **Improvement extraction**: If result contains improvement suggestions, TODOs, or "handle in separate task" items with independent work scope, use `/tq:create-action` to create a follow-up action
3. **Next action decision**: Determine the next action needed to achieve the task's goal:
   - Additional work needed → `/tq:create-action` to create it
   - External blocker (waiting for review, approval, etc.) → do nothing
   - An active action (pending/running) with the same purpose already exists → do not create
4. **Task completion check**: Only if no action was created above, determine whether the task's goal has been achieved → if complete, run `tq task update <task_id> --status done`

Constraints:
- Dedup: Do not create an action if an active action (pending/running) with the same purpose already exists
- If predecessor_result contains incomplete signals, do NOT mark the task as done

## Execute

`tq action cancel <action_id> '<reason>'`
