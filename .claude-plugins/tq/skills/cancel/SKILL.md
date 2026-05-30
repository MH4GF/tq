---
description: Cancel a tq action with improvement suggestions, then judge task-level completion and propose follow-up actions when work remains
argument-hint: "[action_id]"
allowed-tools: Bash(tq *)
---

# tq action cancel

IMPORTANT: Run !`tq action cancel --help` first to understand the expected reason format.

## Find action_id

1. `$ARGUMENTS` if numeric
2. The action/task IDs stated under the `## tq action context` heading appended to the prompt (e.g. "You are executing **action #123** (task #45).")
3. Search non-terminal actions: `tq action list --status running` or `tq action list --status dispatched`
4. If none works, ask the user

## Investigate before cancelling

Review the task's action history to understand why this action was created:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action.

## Settle residual work before cancelling

A cancel reason is mostly classification feedback, but cancelling sometimes
surfaces real work — a different approach to take, an improvement worth
acting on. Same discipline as `/tq:done`: residual work that lives only in a
cancelled action's reason text is invisible to `tq action list` and
`/tq:triage`, so nobody picks it up. Split the reason accordingly:

1. **Classification feedback only.** Why this action was unnecessary, how
   routing could improve. No work is owed — keep it as prose in the reason.
2. **Residual work owed.** A different approach or improvement suggestion
   that someone must actually do. File it as a tracked action and link it:

   ```bash
   tq action create '<self-contained instruction>' --title '<title>' --task <task_id>
   ```

   Reference the returned id in the reason: `next: <what to do> → #<id>`.

Get `<task_id>` from the `## tq action context` heading or
`tq action get <action_id> --jq .task_id`. Write the instruction the way
`/tq:create-action` would — goal-first, self-contained, with verification.
You only have `Bash(tq *)` here, so call `tq action create` directly. If
`tq action create` fails, state in the reason that the follow-up could not be
filed and must be re-filed — never leave residual work as untracked prose.

## Execute

`tq action cancel <action_id> '<reason>'`

## After cancelling: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + review the cancellation reason you just recorded.
2. Classify the task:
   - **Done** — task goal already achieved by other actions, or the task is
     no longer relevant.
   - **Follow-up needed** — speculative task-level next steps not already
     captured as a `→ #<id>` reference (a broader change worth its own task).
     Propose 1–2 candidates (title + one-line purpose) and ask the user to
     create via `/tq:create-action`. Do not auto-create *these* — they are
     judgment calls, unlike the residual-work tracking actions above, which
     you file yourself because they record work you already know is owed.
   - **External blocker only** — the cancellation was due to an external
     dependency (waiting for upstream fix, review, etc.). State explicitly:
     "Task #<id> stays open, waiting on <dep>." If that residue carries
     future work of its own, it must already have its `→ #<id>`.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the cancellation reason or action history contains incomplete signals, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running/dispatched) action with the same purpose already exists for this task.
