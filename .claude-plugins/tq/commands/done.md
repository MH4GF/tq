---
description: Mark a tq action as done, then judge task-level completion and propose follow-up actions when work remains
argument-hint: "<action_id> [summary]"
allowed-tools: Bash(tq *)
---

# tq action done

## Find action_id

1. `$ARGUMENTS` if numeric
2. The action/task IDs stated in the dispatch postamble (e.g. "You are executing action #123 (task #45)")
3. Search non-terminal actions: `tq action list --status running` or `tq action list --status dispatched`
4. If none works, ask the user

## Execute

IMPORTANT: Run !`tq action done --help` for the full result format guidance.

`tq action done <action_id> '<result>'`

Result must use structured sections: outcome, decisions, artifacts, remaining.
Do NOT describe process steps — session logs capture that.

## After marking done: task-level follow-up

Always run this flow — do not wait for the user to ask "what's next?".

1. `tq action list --task <task_id>` + re-read the `remaining` you just wrote.
2. Classify the task:
   - **Done** — no remaining work, no external dependency.
   - **Follow-up needed** — local work still required (address review comments, extract improvement TODOs, etc.). Propose 1–2 next-action candidates (title + one-line purpose) and ask the user to create via `/tq:create-action`. Do not auto-create.
   - **External blocker only** — the residue is a PR merge, review reply, upstream release, or another task. State explicitly: "Task #<id> stays open, waiting on <dep>." Optionally propose a tracking action (e.g. PR-merge follow-up) if none is already queued.
3. Close the task only when classification is **Done**:
   `tq task update <task_id> --status done --note "<why>"` (`--note` required with `--status`).

Constraints:
- If the result's `remaining` section has incomplete signals, classification cannot be **Done**.
- Dedup: skip the proposal if an active (pending/running) action with the same purpose already exists for this task.
