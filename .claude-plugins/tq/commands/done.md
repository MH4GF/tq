---
description: Mark a tq action as done and report results
argument-hint: "<action_id> [summary]"
allowed-tools: Bash(tq *)
---

# tq action done

## Find action_id

1. Environment variable `TQ_ACTION_ID` — always check this first. It is pre-set by the dispatcher and avoids an extra CLI call.
2. Search running actions: `tq action list --status running`
3. If neither works, ask the user

## Next Action

Before marking done, determine if follow-up work is needed:

1. Run `tq action list --task <task_id>` to review action history
2. **Improvement extraction**: If result contains improvement suggestions, TODOs, or "handle in separate task" items with independent work scope, use `/tq:create-action` to create a task
3. **Next action decision**: Determine the next action needed to achieve the task's goal:
   - Additional work needed → `/tq:create-action` to create it
   - External blocker (waiting for review, approval, etc.) → do nothing
   - An active action (pending/running) with the same purpose already exists → do not create
4. **Task completion check**: Only if no action was created above, determine whether the task's goal has been achieved → if complete, run `tq task update <task_id> --status done`

Constraints:
- Dedup: Do not create an action if an active action (pending/running) with the same purpose already exists
- If predecessor_result contains incomplete signals, do NOT mark the task as done

## Execute

IMPORTANT: Run `tq action done --help` for the full result format guidance.

`tq action done <action_id> '<result>'`

Result must use structured sections: outcome, decisions, artifacts, remaining.
Do NOT describe process steps — session logs capture that.
