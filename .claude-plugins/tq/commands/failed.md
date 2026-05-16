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
2. The action/task IDs stated under the `## tq action context` heading appended to the prompt (e.g. "You are executing **action #123** (task #45).")
3. Search non-terminal actions: `tq action list --status running` or `tq action list --status dispatched`
4. If none works, ask the user

## Investigate before failing

Review the task's action history to understand context and confirm failure is the right outcome:

`tq action list --task <task_id>`

Read each action's result to trace the chain of decisions that led to this action. Confirm:
- Was every reasonable approach actually tried?
- Is the blocker truly external (vs. a misunderstanding that more effort could resolve)?
- Would `/tq:done` with a partial-completion summary be more honest?

## Settle `remaining` before failing

Here `remaining` records what is needed to unblock. Same discipline as
`/tq:done`, adapted to that meaning: a retry plan or alternative approach is a
promise nobody can keep if it lives only in a closed action's reason text.
Triage each line into one of two shapes:

1. **Pure external wait — nothing for us to do.** "Blocked until the vendor
   API recovers", "waiting on a permission grant". No session work is owed,
   so it is context, not a tracked item. State it plainly in the reason.
2. **Future session work owed.** Prepare a retry, try a different approach,
   fix the broken setup. File it as a tracked action and link it:

   ```bash
   tq action create '<self-contained instruction>' --title '<title>' --task <task_id>
   ```

   Append the returned id: `- <what to retry / try next> → #<id>`. For
   time-based blockers (API down, CI flake) put the retry timing in the
   follow-up's `--meta` as `dispatch_after`.

Get `<task_id>` from the `## tq action context` heading or
`tq action get <action_id> --jq .task_id`. Write the instruction the way
`/tq:create-action` would — goal-first, self-contained, with verification.
You only have `Bash(tq *)` here, so call `tq action create` directly.

**If `tq action create` fails**, still mark this action failed (the work was
already blocked) but state in the reason that the follow-up could not be filed
and must be re-filed — never leave the retry plan as untracked prose.

## Execute

`tq action fail <action_id> '<structured reason>'`

Reason uses structured sections: outcome, decisions, artifacts, remaining.
Describe the blocker and what was tried, not the process — session logs capture that.

## After failing: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + re-read the `remaining` you just wrote.
2. Classify the task:
   - **Done** — failure revealed the task goal is no longer achievable or
     relevant (e.g. the feature was removed upstream, the problem no longer
     exists).
   - **Follow-up needed** — speculative task-level next steps not already
     captured as a `remaining → #<id>` line (a fundamentally different
     strategy worth its own task, say). Propose 1–2 candidates (title +
     one-line purpose) and ask the user to create via `/tq:create-action`.
     Do not auto-create *these* — they are judgment calls, unlike the
     remaining-entry tracking actions above, which you file yourself because
     they record retry work you already know is owed.
   - **External blocker only** — waiting for external resolution (upstream
     fix, permission grant, review, etc.). State explicitly: "Task #<id>
     stays open, waiting on <dep>." If that residue carries future work of
     its own, it is a `remaining` line and must already have its `→ #<id>`.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the failure reason or action history suggests retry may succeed, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running/dispatched) action with the same purpose already exists for this task.
