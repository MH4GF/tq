---
description: Mark a tq action as failed, then judge task-level completion and propose follow-up actions when retry or alternative approach is needed
argument-hint: "[action_id]"
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
2. The action/task IDs stated in the dispatch postamble (e.g. "You are executing action #123 (task #45)")
3. Search non-terminal actions: `tq action list --status running` or `tq action list --status dispatched`
4. If none works, ask the user

## Investigate before failing

Review the task's action history to understand context and confirm failure is the right outcome:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action. Confirm:
- Was every reasonable approach actually tried?
- Is the blocker truly external (vs. a misunderstanding that more effort could resolve)?
- Would `/tq:done` with a partial-completion summary be more honest?

## Execute

`tq action fail <action_id> '<structured reason>'`

Reason MUST use structured sections: outcome, decisions, artifacts, remaining.
Do NOT describe process steps — session logs capture that.

## After failing: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + re-read the `remaining` you just wrote.
2. Classify the task:
   - **Done** — failure revealed the task goal is no longer achievable or relevant (e.g. the feature was removed upstream, the problem no longer exists).
   - **Follow-up needed** — retry is possible after fixing the blocker, or a different approach should be tried. Propose 1–2 next-action candidates (title + one-line purpose) and ask the user to create via `/tq:create-action`. Do not auto-create. Include `dispatch_after` suggestion for time-based blockers (API down, CI flake).
   - **External blocker only** — waiting for external resolution (upstream fix, permission grant, review, etc.). State explicitly: "Task #<id> stays open, waiting on <dep>." Optionally propose a tracking action if none is already queued.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the failure reason or action history suggests retry may succeed, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running) action with the same purpose already exists for this task.
