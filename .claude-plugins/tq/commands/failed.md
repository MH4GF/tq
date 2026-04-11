---
description: Mark a tq action as failed when the goal could not be achieved
argument-hint: "<action_id>"
allowed-tools: Bash(tq *)
---

# tq action fail

Use this command when an action could not be completed despite genuine effort
(missing permissions, broken environment, external API down, CI flake, etc.).

Distinction from `/tq:done` and `/tq:cancel`:
- `/tq:done` — work completed successfully
- `/tq:failed` — work attempted but blocked; retry may be possible
- `/tq:cancel` — work intentionally aborted (no longer needed, superseded)

IMPORTANT: Run !`tq action fail --help` first to understand the expected reason format.

## Find action_id

1. `$ARGUMENTS` if numeric
2. The action/task IDs stated in the dispatch preamble (e.g. "You are executing action #123 (task #45)")
3. Search running actions: `tq action list --status running`
4. If none works, ask the user

## Investigate before failing

Review the task's action history to understand context and confirm failure is the right outcome:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action. Confirm:
- Was every reasonable approach actually tried?
- Is the blocker truly external (vs. a misunderstanding that more effort could resolve)?
- Would `/tq:done` with a partial-completion summary be more honest?

## Next Action

Before marking failed, determine if follow-up work is needed:

1. Run `tq action list --task <task_id>` to review action history
2. **Retry decision**: Determine the right follow-up based on failure type:
   - **External blocker that may resolve in time** (API down, CI flake, waiting on review) → use `/tq:create-action` to schedule a retry with `dispatch_after` set to a reasonable delay
   - **Environment needs repair** (missing tool, broken config, permission gap) → use `/tq:create-action` to create a repair action; the repair action's completion can trigger the original retry
   - **Truly unrecoverable** → consider whether the parent task should be marked blocked or done; do not silently leave it open
3. **Dedup**: Do not create a follow-up if an active action (pending/running) with the same purpose already exists

## Execute

`tq action fail <action_id> '<structured reason>'`

Reason MUST use structured sections: outcome, decisions, artifacts, remaining.
Do NOT describe process steps — session logs capture that.
