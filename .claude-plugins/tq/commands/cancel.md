---
description: Cancel a tq action with improvement suggestions, then judge task-level completion and propose follow-up actions when work remains
argument-hint: "<action_id>"
allowed-tools: Bash(tq *)
---

# tq action cancel

IMPORTANT: Run !`tq action cancel --help` first to understand the expected reason format.

## Find action_id

1. `$ARGUMENTS` if numeric
2. The action/task IDs stated in the dispatch postamble (e.g. "You are executing action #123 (task #45)")
3. Search running actions: `tq action list --status running`
4. If none works, ask the user

## Investigate before cancelling

Review the task's action history to understand why this action was created:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action.

## Execute

`tq action cancel <action_id> '<reason>'`

## After cancelling: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + review the cancellation reason you just recorded.
2. Classify the task:
   - **Done** — task goal already achieved by other actions, or the task is no longer relevant.
   - **Follow-up needed** — a different approach is required, or improvement suggestions from the cancellation should be acted on. Propose 1–2 next-action candidates (title + one-line purpose) and ask the user to create via `/tq:create-action`. Do not auto-create.
   - **External blocker only** — the cancellation was due to an external dependency (waiting for upstream fix, review, etc.). State explicitly: "Task #<id> stays open, waiting on <dep>." Optionally propose a tracking action if none is already queued.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the cancellation reason or action history contains incomplete signals, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running) action with the same purpose already exists for this task.
